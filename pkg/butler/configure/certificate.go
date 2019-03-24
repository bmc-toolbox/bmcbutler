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
	"strings"
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

	var commonName string
	if b.config.HTTPSCert.Attributes.CommonName == "" {
		return false, fmt.Errorf("Declared certificate configuration requires a commonName")
	}

	commonName = b.config.HTTPSCert.Attributes.CommonName

	if b.butlerConfig.CertSigner == nil {
		return false, fmt.Errorf("No cert signer declared in butler configuration")
	}

	// Retrieve current cert(s)
	certs, csrCapability, err := b.bmc.CurrentHTTPSCert()
	if err != nil {
		return false, fmt.Errorf("Error retreiving current cert: %s", err)
	}

	invalidReason, valid := b.validateCert(certs, b.config.HTTPSCert.Attributes)

	// Compare if the current cert matches declared config.
	if valid {

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
		"Cause":     invalidReason,
	}).Trace("Current certificate does not match configuration.")

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

	// sign CSR
	crt, err := b.signCSR(csr, commonName)
	if err != nil {
		return false, err
	}

	// upload signed cert
	// TODO: This cert format is required only for the Idracs, move into bmclib
	certFileName := fmt.Sprintf("%s.%s", commonName, "crt")

	time.Sleep(time.Second * 1)

	//// TODO:validate stdOut is a PEM block.
	resetBMC, err := b.configure.UploadHTTPSCert(crt, certFileName, privateKey, privateKeyFileName)
	if err != nil {
		return false, fmt.Errorf("Error uploading signed cert: %s", err)
	}

	return resetBMC, nil
}

// signCSR signs the given csr with the configured signer
func (b *Bmc) signCSR(csr []byte, commonName string) ([]byte, error) {

	config := b.butlerConfig.CertSigner

	var cmd string
	var args []string
	var env = make(map[string]string)

	// if we're in trace logging, pass the debugging env var to the signer.
	if b.butlerConfig.Trace {
		env["DEBUG_SIGNER"] = "1"
	}

	// based on configuration, setup cmd, args, env vars
	switch config.Client {
	case "fakeSigner":
		cmd = config.FakeSigner.Bin
		args = config.FakeSigner.Args
		env["PASSPHRASE"] = config.FakeSigner.Passphrase

	case "lemurSigner":
		cmd = config.LemurSigner.Bin
		env["KEY"] = config.LemurSigner.Key
		env["ENDPOINT"] = config.LemurSigner.Endpoint
		a := []string{
			"--valid-years", config.LemurSigner.ValidityYears,
			"--authority", config.LemurSigner.Authority,
			"--owner", config.LemurSigner.Owner,
			"--common-name", commonName,
		}

		args = append(args, a...)
	default:
		return []byte{}, fmt.Errorf("Unknown cert signer declared in butler config")
	}

	if cmd == "" {
		return []byte{}, fmt.Errorf("No signer binary declared in butler config")
	}

	b.logger.WithFields(logrus.Fields{
		"component": "signCSR",
		"signer":    config.Client,
		"cmd":       cmd,
		//"env":       env,
		"args": strings.Join(args, " "),
	}).Trace("Invoked cert signer.")

	// sign the CSR with the configured signer.
	stdOut, stdErr, exitCode := execCmd(cmd, env, args, csr)
	if exitCode != 0 {
		return []byte{}, fmt.Errorf("Error signing CSR: %s", stdErr)
	}

	return []byte(stdOut), nil
}

// Validate a x509 cert attributes with declared configuration
// return a string, bool - based on if the cert attributes aren't valid or is/will expired.
// nolint: gocyclo
func (b *Bmc) validateCert(certs []*x509.Certificate, config *cfgresources.HTTPSCertAttributes) (string, bool) {

	// If there are no certs
	if len(certs) == 0 {
		return "No certs present.", false
	}

	cert := certs[0]

	expires := cert.NotAfter
	if config.RenewBeforeExpiry == 0 {
		config.RenewBeforeExpiry, _ = time.ParseDuration("720h")
	}

	if expires.Sub(time.Now()) < config.RenewBeforeExpiry {
		return fmt.Sprintf("Cert expires in %s", time.Until(expires).String()), false
	}

	// The email address field isn't validated, since HP ILOs don't seem to support it.
	pkix := cert.Subject

	if !match(pkix.Country, config.CountryCode) {
		return fmt.Sprintf("Country Code mismatch, has %s want %s", pkix.Country, config.CountryCode), false

	} else if !match([]string{pkix.CommonName}, config.CommonName) {
		return fmt.Sprintf("CN mismatch, has %s want %s", pkix.CommonName, config.CommonName), false

	} else if !match(pkix.Organization, config.OrganizationName) {
		return fmt.Sprintf("Organization mismatch, has %s want %s", pkix.Organization, config.OrganizationName), false

	} else if !match(pkix.OrganizationalUnit, config.OrganizationUnit) {
		return fmt.Sprintf("OU mismatch, has %s want %s", pkix.OrganizationalUnit, config.OrganizationUnit), false

	} else if !match(pkix.Locality, config.Locality) {
		return fmt.Sprintf("Locality mismatch, has %s want %s", pkix.Locality, config.Locality), false

	} else if !match(pkix.Province, config.StateName) {
		return fmt.Sprintf("Province mismatch, has %s want %s", pkix.Province, config.StateName), false

	} else if len(cert.IPAddresses) < 1 {
		return fmt.Sprintf("Subject Alt Name has no IPAddresses, want %s", b.ip), false

	} else if len(cert.IPAddresses) > 0 {
		if !match([]string{cert.IPAddresses[0].String()}, b.ip) {
			return fmt.Sprintf("Subject Alt Name IPAddress mismatch, has %s want %s", cert.IPAddresses[0].String(), b.ip), false
		}
	}

	return "", true
}

func match(field []string, config string) bool {

	// As of now we don't support > 1 element in the slice
	if len(field) != 1 {
		return false
	}

	if field[0] == config {
		return true
	}

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
