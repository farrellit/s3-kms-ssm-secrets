# s3-kms-ssm-secrets
store secrets of arbitrary size, encrypted with kms data key, in s3 and refer to them from ssm.  Pipe secret data in and out of a commandline binary.  

## Usage

### Parameters

| flag | purpose | valid values (or example values) |
| - | - | - |
| `-O` | Operation | `put` to publish , `get` to retrieve |
| `-p` | Path      | SSM parameter name for `put` operations; SSM parameter or `s3://` URL for `get` operations |
| `-r` | Region    | Region for S3 and SSM - `us-west-2` |
| `-b` | Bucket    | S3 Bucket in which to upload secrets |
| `-k` | Key       | KMS key with which to client-encrypt secrets |
| `-e` | Ext       | File suffix for the S3 Object Key - example: `.tar` |

The content of the secret artifact will come from standard input (`get` operations) or go to standard output (`put` operations).

## Motivation

SSM is a wonderful service, with read-after-write consistency, that can store up to 1k of secret data and allows 20,000 free api calls a month.  
Its principle limitation is that it can't store large blobs of data.

S3 is also a wonderful service, able to store arbitrary amounts of data cheaply and retrieve very quickly at almost unlimited scale.  
Its principle deficiency is eventual consistency due to its caching layer.

Finally, KMS is a high quality key service capable of providing data keys that can be used to encrypt arbitrary blobs of data.

Put them together and what do you get?  Read-after-write consistent ssm references to immutable S3 objects which have themlselves been encrypted with KMS keys.

The `secrets.go` code defined here can do just that.  By piping data in and providing required arguments, you end up with an s3 key 
generated from the given path and the shasum of the data.  The data will be stored in a temporary file so arbitrary sizes can be handled.  The code can then 
be run in reverse, reading the ssm secret to find the s3 key, and then reading it, decrypting it, and writing to stdout.

## Why not just use s3?

Because s3 is not read-after-write consistent.  This may surprise many but it is in fact true.  Perhaps one tenth of one percent of the time 
you can get the old data.  It could even take up to 24 hours to retrieve the new version - though likely it will take seconds or less. 

## Why not just use SSM? 

This works fine if the secret is small enough.  But if the secret is larger than 1k, or 8k for the high tier (and higher cost) Advanced SSM option, 
the secret simply doesn't fit in SSM and you need somewhere else to store the secret.
