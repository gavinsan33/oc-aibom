# oc aibomrun export

```sh
oc aibomrun export <name>
oc aibomrun export <name> | jq '.components[] | select(.type == "library") | .purl'
```

Prints one AIBOM as a [CycloneDX 1.6](https://cyclonedx.org/docs/1.6/json/)
JSON document on stdout, for tools that read software and ML BOMs
(Dependency-Track, GUAC, Lightwell). Nothing is written to the cluster or to
disk.

The mapping follows aibom-webhook-service's
[CycloneDX mapping](https://gavinsan33.github.io/project-aibom/aibom-webhook-service/reference/cyclonedx-mapping/),
which lists every record field with where it goes. Most fields have no native
CycloneDX place and become `aibomrun:*` properties. The record itself stays
the signed source; the document is derived from it and unsigned. It carries
the record's signature and a SHA-256 of the record's canonical form in an
`attestation` reference, so a reader can check it against the record.

The record's signature is checked first, as in `describe`. If it isn't
verified, a warning goes to stderr and the document is still printed, so
stdout is always just the JSON.
