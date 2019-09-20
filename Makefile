test:
	go fmt secrets.go
	go build secrets.go
	set +e; \
	tmpfile="$$(mktemp)"; \
	trap "rm $$tmpfile" EXIT; \
	dd if=/dev/urandom of=$${tmpfile} bs=1 count=32; \
	echo "hi world"; \
	for i in `seq 1 2`; do \
		./secrets -O put -b farrellit-us-east-2 -p /path/to/secret -r us-east-2 -k arn:aws:kms:us-east-2:122377349983:key/646668c4-175b-4158-b55e-56cc8dbda6f7 < $${tmpfile}; \
		[[ "$$(shasum -a 256 < $${tmpfile})" = "$$(./secrets -O get -p /path/to/secret -r us-east-2 | shasum -a 256)" ]] || { echo "Failed get comparison"; exit 1; } \
	done

publish:
	GOOS=linux GOARCH=amd64 go build -o secrets.linux-amd64 secrets.go
	aws s3 cp --region us-west-2 --profile dodsandbox --acl public-read secrets.linux-amd64 s3://env-sandbox-secretsbucket-vcuhbvikk21y/getsecrets/linux/amd64/$$(shasum -a 256 < secrets.linux-amd64 | awk '{print $$1}')/getsecrets
