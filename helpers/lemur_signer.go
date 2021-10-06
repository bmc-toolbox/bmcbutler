package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"
)

// Request is the CSR POST payload
type Request struct {
	Owner         string `json:"owner"`
	CommonName    string `json:"commonName"`
	ValidityYears string `json:"validityYears"`
	Notify        bool   `json:"notify"`
	Key           string `json:"-"`
	Endpoint      string `json:"-"`
	Debug         bool   `json:"-"`
	CSR           string `json:"csr"`
	Authority     `json:"authority"`
	Extensions    `json:"extensions"`
}

// Extensions is part of the Request payload
type Extensions struct {
	SubjectAltNames `json:"subAltNames"`
}

// SubjectAltNames is part of the Request payload
type SubjectAltNames struct {
	Names []Names `json:"names"`
}

// Names is part of the Request payload
type Names struct {
	NameType string `json:"nameType"`
	Value    string `json:"value"`
}

// Response struct is the response payload.
type Response struct {
	Chain string `json:"chain"`
	Body  string `json:"body"`
}

// Authority is part of the Request payload
type Authority struct {
	Name string `json:"name"`
}

func main() {
	var csr []byte
	debug := os.Getenv("DEBUG_SIGNER")
	endpoint := os.Getenv("ENDPOINT")
	key := os.Getenv("KEY")

	commonName := flag.String("common-name", "", "lemur CSR commonName attribute.")
	authority := flag.String("authority", "", "lemur CSR authority attribute.")
	owner := flag.String("owner", "", "lemur CSR payload email address attribute.")
	validYears := flag.String("valid-years", "1", "The time period this certificate should be valid for (default 1 year).")
	notify := flag.Bool("notify", false, "Whether you should get notified about certificate expiration.")

	flag.Parse()

	if key == "" {
		log.Fatal("Expected KEY env variable not set.")
	}

	if endpoint == "" {
		log.Fatal("Expected ENDPOINT env variable not set.")
	}

	if *commonName == "" {
		log.Fatal("Expected CN env variable not set.")
	}

	if *authority == "" {
		log.Fatal("Expected --authority flag not declared.")
	}

	if *owner == "" {
		log.Fatal("Expected --owner flag not declared.")
	}

	if *validYears == "" {
		log.Fatal("Expected valid years to be > 0.")
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		csr = append(csr, []byte(fmt.Sprintln(scanner.Text()))...)
	}

	extensions := Extensions{SubjectAltNames: SubjectAltNames{Names: []Names{{NameType: "DNSName", Value: *commonName}}}}

	a := Authority{Name: *authority}
	r := Request{
		Owner:         *owner,
		CommonName:    *commonName,
		Authority:     a,
		ValidityYears: *validYears,
		Notify:        *notify,
		Key:           key,
		Endpoint:      endpoint,
		CSR:           string(csr),
		Extensions:    extensions,
	}

	if debug == "1" {
		r.Debug = true
	}

	crt, err := sign(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(crt)
	os.Exit(0)
}

// sign returns the signed cert, it will prepend a chain cert if any.
// nolint: gocyclo
func sign(request Request) (string, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return "", err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Timeout: time.Second * 5, Transport: tr}
	req, err := http.NewRequest("POST", request.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	bearer := "Bearer " + request.Key
	req.Header.Add("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")

	if request.Debug {
		dump, err := httputil.DumpRequestOut(req, true)
		if err == nil {
			log.Println(">>>>>>>>>>>>>>>")
			log.Printf("%s\n\n", dump)
			log.Println(">>>>>>>>>>>>>>>")
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Lemur API returned non 200 response: %d, response body: %s", resp.StatusCode, string(buf))
	}

	if request.Debug {
		dump, err := httputil.DumpResponse(resp, true)
		if err == nil {
			log.Println("[Response]")
			log.Println("<<<<<<<<<<<<<<")
			log.Printf("%s\n\n", dump)
			log.Println("<<<<<<<<<<<<<<")
		}
	}

	var response Response
	err = json.Unmarshal(buf, &response)
	if err != nil {
		return "", err
	}

	var chain string
	if response.Body != "" {
		chain = response.Body
		chain += "\n"
	}

	if response.Chain != "" {
		chain += response.Chain
		chain += "\n"
	}

	return chain, nil
}
