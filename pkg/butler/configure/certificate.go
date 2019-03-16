package configure

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"

	"github.com/bmc-toolbox/bmclib/cfgresources"
	"github.com/sirupsen/logrus"
)

// 1. Get current certificate info
// 2. Determine if certificate needs to be updated
// 3. If update required, generate CSR from the BMC
// 4. Get certificate signed by CA service
// 5. Upload signed certificate on the BMC.
// iDrac needs a reset
// POST https://10.193.251.25/data?set=iDracReset:1
func (b *Bmc) certificateSetup() (bool, error) {

	// Retrieve current cert(s)
	certs, csrCapability, err := b.bmc.CurrentHTTPSCert()
	if err != nil {
		return false, fmt.Errorf("Error retreiving current cert: %s", err)
	}

	// Compare if the current cert matches declared config.
	if b.certMatchConfig(certs, b.config.HTTPSCert.Attributes) {

		b.logger.WithFields(logrus.Fields{
			"Vendor":    b.vendor,
			"Model":     b.model,
			"Serial":    b.serial,
			"IPAddress": b.ip,
		}).Trace("Current certificate matches configuration.")

		return false, nil
	}

	b.logger.WithFields(logrus.Fields{
		"Vendor":    b.vendor,
		"Model":     b.model,
		"Serial":    b.serial,
		"IPAddress": b.ip,
	}).Trace("Current certificate to be updated.")

	var csr []byte
	var privateKey []byte
	var privateKeyFileName string

	// BMC doesn't support generating a CSR
	if !csrCapability {
		// Generate a CSR locally
		csr, privateKey, err = generateCsr(b.config.HTTPSCert.Attributes)
	} else {
		// Generate a CSR on the BMC
		csr, err = b.configure.GenerateCSR(b.config.HTTPSCert.Attributes)
	}

	if err != nil {
		return false, fmt.Errorf("CSR not generated: %s", err)
	}

	// sign the CSR with the configured signer.
	cmd := b.butlerConfig.SignerParams.Bin
	args := b.butlerConfig.SignerParams.Args
	env := map[string]string{"PASSPHRASE": b.butlerConfig.SignerParams.Passphrase}

	stdOut, stdErr, exitCode := execCmd(cmd, env, args, csr)
	if exitCode != 0 {
		return false, fmt.Errorf("Error signing CSR: %s", stdErr)
	}

	// upload signed cert
	certFileName := fmt.Sprintf("%s.%s", b.config.HTTPSCert.Attributes.CommonName, "crt")

	time.Sleep(time.Second * 1)

	//// TODO:validate stdOut is a PEM block.
	resetBMC, err := b.configure.UploadHTTPSCert([]byte(stdOut), certFileName, privateKey, privateKeyFileName)
	if err != nil {
		return false, fmt.Errorf("Error uploading signed cert: %s", err)
	}

	return resetBMC, nil
}

// TODO
// Write this method which will compare attributes.
func (b *Bmc) certMatchConfig(certs []*x509.Certificate, config *cfgresources.HTTPSCertAttributes) bool {

	// If there are no certs
	if len(certs) == 0 {
		return false
	}

	cert := certs[0]

	pkix := cert.Subject

	if !match(pkix.Country, config.CountryCode) {
		return false
	} else if !match(pkix.Country, config.CountryCode) {
		return false
	} else if !match([]string{pkix.CommonName}, config.CommonName) {
		return false
	} else if !match(pkix.Organization, config.OrganizationName) {
		return false
	} else if !match(pkix.OrganizationalUnit, config.OrganizationUnit) {
		return false
	} else if !match(pkix.Locality, config.Locality) {
		return false
	} else if !match(pkix.Province, config.StateName) {
		return false
	} else if len(cert.IPAddresses) < 1 {
		return false
	} else if len(cert.IPAddresses) > 0 {
		if !match([]string{cert.IPAddresses[0].String()}, b.ip) {
			return false
		}
	}

	return true
}

func match(field []string, config string) bool {

	// As of now we don't support > 1 element in the slice
	if len(field) != 1 {
		//fmt.Printf(">> %+v\n", field)
		return false
	}

	if field[0] == config {
		//fmt.Printf(">>>%s == %s<<\n", field[0], config)
		return true
	}

	//fmt.Printf(">>>%s != %s<<\n", field[0], config)
	return false
}

// run command with args
func execCmd(c string, env map[string]string, args []string, stdIn []byte) (stdOut string, stdErr string, exitCode int) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c, args...)

	// setup the stdin/stdout buffers
	var outBuf, errBuf bytes.Buffer

	// if there are env variables declared in the checks config,
	// set them up in the command environment.
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// setup output/input redirections
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	//feed in given stdin data
	cmd.Stdin = bytes.NewBuffer(stdIn)

	// To ignore SIGINTs received by the parent process,
	// this is to allow watson to gracefully handle ongoing goroutines,
	// this causes the commands to be spawned in its own process group.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// run command
	err := cmd.Run()
	stdOut = outBuf.String()
	stdErr = errBuf.String()

	// check if cmd.Run returned an error
	if err != nil {
		// check if we have an exit error
		exitError, ok := err.(*exec.ExitError)
		if !ok {
			// if we do not have an exit error, we return 1
			exitCode = 1
			if len(stdErr) == 0 {
				stdErr = err.Error()
			}
		} else {
			ws := exitError.Sys().(syscall.WaitStatus)
			exitCode = ws.ExitStatus()
		}
	} else {
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
	}

	return stdOut, stdErr, exitCode
}

func generateCsr(c *cfgresources.HTTPSCertAttributes) (csr, privateKey []byte, err error) {

	// https://oidref.com/1.2.840.113549.1.9.1
	var oidEmailAddress = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 1}

	// Generate private key
	keyBytes, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return csr, privateKey, err
	}

	// fill in the Subject values
	subject := pkix.Name{
		CommonName:         c.CommonName,
		Country:            []string{c.CountryCode},
		Province:           []string{c.StateName},
		Locality:           []string{c.Locality},
		Organization:       []string{c.OrganizationName},
		OrganizationalUnit: []string{c.OrganizationUnit},
	}

	// Append Email address
	rawSubject := subject.ToRDNSequence()
	rawSubject = append(rawSubject, []pkix.AttributeTypeAndValue{
		{Type: oidEmailAddress, Value: c.Email},
	})

	asn1Subj, _ := asn1.Marshal(rawSubject)
	if err != nil {
		return csr, privateKey, err
	}

	// Build the CSR template
	template := x509.CertificateRequest{
		RawSubject:         asn1Subj,
		EmailAddresses:     []string{c.Email},
		SignatureAlgorithm: x509.SHA256WithRSA,
	}

	// Add IPaddress
	// TODO: identify if its an IP or a A record
	template.IPAddresses = []net.IP{net.ParseIP(c.SubjectAltName)}

	// Generate csr
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, keyBytes)
	if err != nil {
		return csr, privateKey, err
	}

	// PEM encode private key block
	privateKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(keyBytes)})

	// PEM encode CSR block
	csr = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})

	return csr, privateKey, err

}
