module github.com/doctorondemand/s3-kms-ssm-secrets

replace github.com/doctorondemand/s3-kms-ssm-secrets/secrets => ./secrets

go 1.13

require (
	github.com/aws/aws-sdk-go v1.34.11
	github.com/doctorondemand/s3-kms-ssm-secrets/secrets v0.0.0-00010101000000-000000000000 // indirect
	github.com/jessevdk/go-flags v1.4.0
)
