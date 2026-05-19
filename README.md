# workflow-plugin-gcp

> ⚠️ **Experimental** — This plugin compiles and passes its unit tests but has not been validated in any active GoCodeAlone-internal production deployment. Use with caution. Please [open an issue](https://github.com/GoCodeAlone/workflow-plugin-gcp/issues/new) if you adopt it so we can promote it to **verified** status.

[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/GoCodeAlone/workflow-plugin-gcp.svg)](https://pkg.go.dev/github.com/GoCodeAlone/workflow-plugin-gcp)

GCP provider plugin for workflow IaC — manages Cloud Run, GKE, Cloud SQL, Memorystore, VPC, Load Balancer, Cloud DNS, Artifact Registry, API Gateway, Firewall, IAM roles, Cloud Storage, and SSL certificates.

## What it provides

**Module types:**
- `iac.provider` — GCP infrastructure provider
- `gcp.credentials` — GCP credentials configuration
- `storage.gcs` — Google Cloud Storage module

**IaC state backends:**
- `gcs` — Google Cloud Storage state backend

## Install

```yaml
# In your wfctl.yaml
version: 1
plugins:
  - name: workflow-plugin-gcp
    version: v2.0.0
    source: github.com/GoCodeAlone/workflow-plugin-gcp
```

Then:
```sh
wfctl plugin install
```

## Minimal example

See `examples/minimal/config.yaml`.

Authentication: Application Default Credentials by default; or set `GOOGLE_APPLICATION_CREDENTIALS` (path to a service account JSON file). Set `GCP_PROJECT` for use in config templates.

## Documentation

- [Plugin authoring guide (upstream)](https://github.com/GoCodeAlone/workflow/blob/main/docs/PLUGIN_AUTHORING.md)
- [Workflow engine docs](https://github.com/GoCodeAlone/workflow)

## License

MIT. See [LICENSE](LICENSE).
