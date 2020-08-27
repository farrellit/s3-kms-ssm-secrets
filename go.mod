module github.com/doctorondemand/s3-kms-ssm-secrets

replace github.com/doctorondemand/dod-config/secrets => ./secrets

go 1.13

require (
	github.com/aws/aws-sdk-go v1.34.10
	github.com/jessevdk/go-flags v1.4.0
)
