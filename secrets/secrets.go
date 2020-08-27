
package secrets

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/kms"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3crypto"
	"github.com/aws/aws-sdk-go/service/ssm"
	"io"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
)

type S3SSMSecret struct {
	Region string
	Path   string
	Bucket string
	Key    string
	sess   *session.Session
	awscfg *aws.Config
	s3c    *s3.S3
	ssmc   *ssm.SSM
	tmpf   *os.File
}

func (s5 *S3SSMSecret) Initialize() {
	s5.awscfg = aws.NewConfig().WithRegion(s5.Region)
	s5.sess = session.Must(session.NewSession(s5.awscfg))
	s5.s3c = s3.New(s5.sess)
	s5.ssmc = ssm.New(s5.sess)
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
	s3url := fmt.Sprintf("s3://%s", path.Join(s5.Bucket, s3objkey))
	if !s5.ObjectExists(s3objkey) {
		handler := s3crypto.NewKMSKeyGenerator(kms.New(s5.sess), s5.Key)
		svc := s3crypto.NewEncryptionClient(s5.sess, s3crypto.AESGCMContentCipherBuilder(handler))
		putres, err := svc.PutObject(&s3.PutObjectInput{
			Bucket: aws.String(s5.Bucket),
			Key:    aws.String(s3objkey),
			Body:   s5.tmpf,
		})
		if err != nil {
			log.Fatal("Couldn't PutObject to S3:", err)
		}
		log.Printf("Object created at %s, etag %s", s3url, aws.StringValue(putres.ETag))
	} else {
		log.Printf("Object at %s already exists, with equal shasum (%s), and is assumed to be the same", s3url, shasum)
	}
	// put in ssm
	if _, err = s5.ssmc.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(s5.Path),
		Description: aws.String("Pointer to s3 object"),
		Overwrite:   aws.Bool(true),
		Value:       aws.String(s3url),
		Type:        aws.String("String"),
	}); err != nil {
		log.Fatal("Couldn't put ssm: ", err)
	}
	log.Printf("Put s3 URL %s in ssm under %s", s3url, s5.Path)
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
	var fullurl string
	if strings.HasPrefix(s5.Path, "s3://") {
		log.Println("Path starts with s3:// - no ssm param needs to be retrieved (this is primarily meant for cloudformation ssm parameters)")
		fullurl = s5.Path
	} else {
		ssmparam, err := s5.ssmc.GetParameter(&ssm.GetParameterInput{
			Name:           aws.String(s5.Path),
			WithDecryption: aws.Bool(true),
		})
		if err != nil {
			log.Fatal(err)
		}
		fullurl = aws.StringValue(ssmparam.Parameter.Value)
	}
	parsedurl, err := url.Parse(fullurl)
	if err != nil {
		log.Fatal("Couldn't parse s3 url", err)
	}
	// TODO: Bucket should be part of path
	svc := s3crypto.NewDecryptionClient(s5.sess)
	obj, err := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(parsedurl.Host),
		Key:    aws.String(parsedurl.Path),
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
	log.Print("wrote s3 content to output")
	return
}

