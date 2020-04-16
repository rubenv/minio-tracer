# minio-tracer

> Finds any Minio server running in a Kubernetes cluster and outputs the call trace

[![Build Status](https://github.com/rubenv/minio-tracer/workflows/Publish/badge.svg)](https://github.com/rubenv/minio-tracer/actions)

## Usage

```
helm repo add minio-tracer https://rubenv.github.io/minio-tracer
helm install minio-tracer minio-tracer/minio-tracer
```

Should be installed in the same namespace as Minio.

## Configuration

The following table lists the configurable parameters of the minio-tracer chart and their default values.

Parameter | Description | Default
--- | --- | ---
`secretName` | name of the secret that Minio uses for credentials | `minio`
`serviceName` | name of the service that Minio uses | `minio`

## License

This app is distributed under the [MIT](LICENSE) license.
