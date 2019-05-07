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

type S3SSMSecret struct {
	Region string
	Path   string
	Bucket string
	sess   *session.Session
	awscfg *aws.Config
	s3c    *s3.S3
	ssmc   *ssm.SSM
	tmpf   *os.File
}

func (s5 *S3SSMSecret) Initialize() {
	s5.sess = session.Must(session.NewSession())
	s5.awscfg = aws.NewConfig().WithRegion(s5.Region)
	s5.s3c = s3.New(s5.sess, s5.awscfg)
	s5.ssmc = ssm.New(s5.sess, s5.awscfg)
}

func (s5 *S3SSMSecret) Put(in *os.File) (s3objkey string, err error) {
	if s5.tmpf, err = ioutil.TempFile("", "*"); err != nil {
		log.Fatal("Temporary file could not be created: ", err)
	}
	// remove now for some semblance of security
	if err = os.Remove(s5.tmpf.Name()); err != nil {
		log.Fatalf("Could not delete temporary file %s: %s", s5.tmpf.Name(), err)
	}
	// TODO: custom writer to stream only once to both places
	// TODO: also sending to GCS might be nice
	io.Copy(s5.tmpf, in)
	s5.tmpf.Seek(0, os.SEEK_SET)
	sha := sha256.New()
	io.Copy(sha, s5.tmpf)
	shasum := hex.EncodeToString(sha.Sum(nil))
	log.Println(os.Stderr, "Input shasum is ", shasum)
	s5.tmpf.Seek(0, os.SEEK_SET)
	s3objkey = path.Join(s5.Path, shasum)
	if !s5.ObjectExists(s3objkey) {
		// TODO: encrypt object
		s5.s3c.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(s5.Bucket),
			Key:    aws.String(s3objkey),
			Body:   s5.tmpf,
		})
		log.Printf("Object created at s3://%s", path.Join(s5.Bucket, s3objkey))
	}
	// put in ssm
	if _, err = s5.ssmc.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(s5.Path),
		Description: aws.String("Pointer to s3 object"),
		Overwrite:   aws.Bool(true),
		Value:       aws.String(s3objkey),
		Type:        aws.String("String"),
	}); err != nil {
		log.Fatal("Couldn't put ssm: ", err)
	}
	log.Printf("Put s3 key s3://%s in ssm under %s", path.Join(s5.Bucket, s3objkey), s5.Path)
	return
}

func (s5 *S3SSMSecret) ObjectExists(s3objkey string) (objexists bool) {
	objexists = false
	s3head, err := s5.s3c.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(s5.Bucket),
		Key:    aws.String(s3objkey),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NotFound" {
			log.Printf("Object s3://%s/%s not found", s5.Bucket, s3objkey)
		} else {
			log.Fatal("Could not HEAD object: ", err)
		}
	} else {
		log.Println("HEAD object: length ", aws.Int64Value(s3head.ContentLength), " etag ", aws.StringValue(s3head.ETag))
		objexists = true
	}
	return
}

func (s5 *S3SSMSecret) Get(out *os.File) (s3key string, err error) {
	ssmparam, err := s5.ssmc.GetParameter(&ssm.GetParameterInput{
		Name:           aws.String(s5.Path),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		log.Fatal(err)
	}
	s3key = aws.StringValue(ssmparam.Parameter.Value)
	// TODO: Bucket should be part of path
	obj, err := s5.s3c.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s5.Bucket),
		Key:    aws.String(s3key),
	})
	if err != nil {
		log.Fatalf("Could not get s3 path s3://%s from parameter %s: %s",
			path.Join(s5.Bucket, s3key),
			s5.Path,
			err,
		)
	}
	defer obj.Body.Close()
	io.Copy(out, obj.Body)
	return
}

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
	s5 := &S3SSMSecret{Region: opts.Region, Path: opts.Path, Bucket: opts.Bucket}
	s5.Initialize()
	if opts.Op == "put" {
		s5.Put(os.Stdin)
	} else {
		s5.Get(os.Stdout)
	}
}
