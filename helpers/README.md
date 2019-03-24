#### Lemur signer bin

The lemur signer expects a csr via stdin, along with the ENV vars and CLI flags,
it passes the cert to the declared lemur endpoint, 
once done, it spits out the signed cert chain at stdout.

Expects the [netfix lemur](https://github.com/Netflix/lemur) API to be available.

CLI Args (all required)
--------

Authority to use for signing

`--authority="TestCA"`

Cert validity in years

`--valid-years 1`

Owner contact email address

`--owner admin@example.com`

Common Name (required)

`--common-name xadse231.bmc.example.com`

ENV variables 
-------------

Lemur auth token

`KEY=sdfgsASDWERsdfsdfwersfdgsfg`

Lemur API endpoint 

`ENDPOINT=https://lemur/api/1/certificates`

Sample request
--------------
```
  cat /tmp/idrac9.csr | DEBUG_SIGNER=1 KEY="sdfgsASDWERsdfsdfwersfdgsfg" \
                        ENDPOINT="https://lemur/api/1/certificates" ./lemur_signer \
                       --authority="TestCA" --valid-years 1 --owner joel.rebello@booking.com --common-name="xadse231.bmc.example.com"
```
