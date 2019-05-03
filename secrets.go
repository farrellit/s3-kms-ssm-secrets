package main

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/jessevdk/go-flags"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
)

func main() {
	var opts struct {
		Region string `short:"r" long:"region" description:"aws region" required:"t"`
		Path   string `short:"p" long:"path" description:"path for secret in ssm, and (with shasum) s3" required:"t"`
		Bucket string `short:"b" long:"bucket" description:"bucket in which to place secrets" required:"t"`
		Op     string `short:"O" long:"operation" description:"operation, get or put" choice:"get" choice:"put" required:"t"`
	}
	if _, err := flags.Parse(&opts); err != nil {
		panic(err)
	}
	// write stdin to temp file to handle arbitrary sizes
	var tmpf *os.File
	var err error
	if tmpf, err = ioutil.TempFile("", "*"); err != nil {
		log.Fatal("Temporary file could not be created: ", err)
	}
	// remove now for some semblance of security
	if err := os.Remove(tmpf.Name()); err != nil {
		log.Fatalf("Could not delete temporary file %s: %s", tmpf.Name(), err)
	}
	// TODO: custom writer to stream only once to both places
	io.Copy(tmpf, os.Stdin)
	tmpf.Seek(0, os.SEEK_SET)
	sha := sha256.New()
	io.Copy(sha, tmpf)
	shasum := hex.EncodeToString(sha.Sum(nil))
	log.Println(os.Stderr, "Input shasum is ", shasum)
	tmpf.Seek(0, os.SEEK_SET)
	// check file location in s3
	sess := session.Must(session.NewSession())
	awscfg := aws.NewConfig().WithRegion(opts.Region)
	s3c := s3.New(sess, awscfg)
	s3objkey := path.Join(opts.Path, shasum)
	s3head, err := s3c.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(opts.Bucket),
		Key:    aws.String(s3objkey),
	})
	objexists := false
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			log.Println("Ojbect not found")
		} else {
			log.Fatal("Could not HEAD object: ", err)
		}
	} else {
		log.Println("HEAD object: length ", s3head.ContentLength, " etag ", s3head.ETag)
		objexists = true
	}
	if objexists == false {
		// TODO: encrypt object
		s3c.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(opts.Bucket),
			Key:    aws.String(s3objkey),
			Body:   tmpf,
		})
		log.Printf("Object created at s3://%s/%s", opts.Bucket, s3objkey)
	}
	// put in ssm
	ssmc := ssm.New(sess, awscfg)
	if _, err := ssmc.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(opts.Path),
		Description: aws.String("Pointer to s3 object"),
		Overwrite:   aws.Bool(true),
		Value:       aws.String(s3objkey),
		Type:        aws.String("String"),
	}); err != nil {
		log.Fatal("Couldn't put ssm: ", err)
	}
	log.Printf("Put s3 key %s in ssm under %s", s3objkey, opts.Path)
}
