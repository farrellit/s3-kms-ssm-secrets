package main

import (
	"github.com/doctorondemand/s3-kms-ssm-secrets/secrets"
	"github.com/jessevdk/go-flags"
	"log"
	"os"
)

func main() {
	var opts struct {
		Region string `short:"r" long:"region" description:"aws region" required:"t"`
		Path   string `short:"p" long:"path" description:"path for secret in ssm, or direct s3 link to be used with cloudformation ssm input" required:"t"`
		Bucket string `short:"b" long:"bucket" description:"bucket in which to place secrets"`
		Key    string `short:"k" long:"key" description:"key with which to client-encrypt secrets"`
		Op     string `short:"O" long:"operation" description:"operation, get or put" choice:"get" choice:"put" required:"t"`
		Ext    string `short:"e" long:"extenion" description:"extension, file suffix"`
	}
	if _, err := flags.Parse(&opts); err != nil {
		log.Fatal("Could not parse command line options:", err)
	}
	if opts.Op == "put" {
		if opts.Bucket == "" {
			log.Fatal("On put operations, bucket command line option must be specified")
		}
		if opts.Key == "" {
			log.Fatal("On put operations, key command line option must be specified")
		}
	} else if opts.Op == "get" {
		if opts.Bucket != "" {
			log.Fatal("On get operations, bucket comes from settings value and cannot be specified as a command line option")
		}
	}
	s5 := &secrets.S3SSMSecret{
		Region: opts.Region,
		Path:   opts.Path,
		Bucket: opts.Bucket,
		Key:    opts.Key,
		Ext:    opts.Ext,
	}
	s5.Initialize()
	if opts.Op == "put" {
		s5.Put(os.Stdin)
	} else {
		s5.Get(os.Stdout)
	}
}
