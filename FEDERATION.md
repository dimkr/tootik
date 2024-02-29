# Federation

## HTTP Signatures

* tootik implements [draft-cavage-http-signatures-12](https://datatracker.ietf.org/doc/html/draft-cavage-http-signatures) but only partially:
  * It ignores query
  * It always uses `rsa-sha256`, ignores `algorithm` and puts `algorithm="hs2019"` in outgoing requests
  * It validates `Host`, `Date` (see `MaxRequestAge`) and `Digest`
  * Validation ensures that key size is between 2048 and 8192
  * Incoming `POST` requests must have at least `headers="(request-target) host date digest"`
  * All other incoming requests must have at least `headers="(request-target) host date"`
  * Outgoing `POST` requests have `headers="(request-target) host date content-type digest"`
  * All other outgoing requests have `headers="(request-target) host date"`
