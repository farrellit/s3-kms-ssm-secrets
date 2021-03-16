test:
	go fmt secrets.go
	go build -o secrets.local secrets.go
	set +e; \
	export AWS_SDK_LOAD_CONFIG=true; \
	tmpfile="$$(mktemp)"; \
	trap "rm $$tmpfile" EXIT; \
	dd if=/dev/urandom of=$${tmpfile} bs=1 count=32; \
	sha256="$$(shasum -a 256 < $${tmpfile} | awk '{print $$1}')"; \
	echo "hi world, shasum is $$sha256"; \
	account=$$(aws sts get-caller-identity --query Account --output text); echo $$account; \
	for i in `seq 1 2`; do \
		./secrets.local -O put -b farrellit-us-east-2 -p /path/to/secret -r us-east-2 -k arn:aws:kms:us-east-2:122377349983:key/646668c4-175b-4158-b55e-56cc8dbda6f7 < $${tmpfile}; \
		[[ "$$(shasum -a 256 < $${tmpfile})" = "$$(./secrets.local -O get -p /path/to/secret -r us-east-2 | shasum -a 256)" ]] || { echo "Failed get comparison"; exit 1; }; \
	done ; \
	[[ "$$(shasum -a 256 < $${tmpfile})" = "$$(./secrets.local -O get -p s3://farrellit-us-east-2/path/to/secret/$$sha256  -r us-east-2 | shasum -a 256)" ]] || { echo "Failed get comparison"; exit 1; };

publish:
	GOOS=linux GOARCH=amd64 go build -o secrets.linux-amd64 secrets.go
	aws s3 cp --acl public-read secrets.linux-amd64 s3://$${publish_bucket}/getsecrets/linux/amd64/$$(shasum -a 256 < secrets.linux-amd64 | awk '{print $$1}')/getsecrets
