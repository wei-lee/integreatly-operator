package providers

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/pkg/errors"
	"regexp"
)

// TODO this should be sourced from where the cluster is created.
const region = "eu-west-1"

type AWSServices struct {
	awsSession *session.Session
}

func NewAWSServices() (*AWSServices, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	return &AWSServices{awsSession: sess}, nil
}

func (awsServ *AWSServices) CreateCloudStorage(name string) error {
	s3Api := s3.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	bucket := s3.CreateBucketInput{
		Bucket: aws.String(name),
		CreateBucketConfiguration: &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(region),
		},
	}
	_, err := s3Api.CreateBucket(&bucket)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeBucketAlreadyExists, s3.ErrCodeBucketAlreadyOwnedByYou:
				return newAlreadyExistsError("s3 bucket " + name + ": " + err.Error())
			default:
				return errors.Wrap(err, "failed to create s3 bucket "+name)
			}
		} else {
			return errors.Wrap(err, "failed to create s3 bucket "+name)
		}
	}
	return nil
}

func (awsServ *AWSServices) RemoveCloudStorage(name string) error {
	fmt.Println("remove cloud storage")
	s3Api := s3.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	bucket := s3.DeleteBucketInput{
		Bucket: aws.String(name),
	}
	if _, err := s3Api.DeleteBucket(&bucket); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket:
				return newDoesntExistErr("s3 bucket " + name + " in region " + region + " : " + err.Error())
			default:
				return errors.Wrap(err, "failed to delete s3 bucket "+name)
			}

		} else {
			return errors.Wrap(err, "failed to delete s3 bucket "+name)
		}
	}
	return nil
}

func (awsServ *AWSServices) ListCloudStorage() error {
	fmt.Println("list cloud storage")
	s3Api := s3.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	bucket := s3.ListBucketsInput{}

	l, err := s3Api.ListBuckets(&bucket)
	if err != nil {
		return errors.Wrap(err, "failed to list s3 buckiets")

	}
	for _, b := range l.Buckets{
		fmt.Println("bucket ", b.String())
	}
	return nil
}

func (awsServ *AWSServices)CreateCloudDB(c CloudDB)(*CloudDBCoords, error)  {
	fmt.Println("create cloud db ")
	rdsApi := rds.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	dbIn := &rds.CreateDBInstanceInput{}
	dbIn.AllocatedStorage = &c.StorageSize
	minorUpgrade := true
	dbIn.AutoMinorVersionUpgrade = &minorUpgrade
	dbIn.BackupRetentionPeriod = &c.RetentionPeriod
	// TODO will need to be decided based on cluster type (dev, prod)
	instClass := "db.t2.medium"
	dbIn.DBInstanceClass = &instClass
	dbIn.DBName = &c.DBName
	reg := regexp.MustCompile("\\s+")
	c.ClusterName = reg.ReplaceAllString(c.ClusterName, "-")
	dbIn.DBInstanceIdentifier = &c.ClusterName
	var engine string
	if c.Engine == CloudDbEnginePostgres {
		engine = "postgres"
		dbIn.Engine = &engine
	}
	dbIn.EngineVersion   = &c.EngineVersion
	public := false
	dbIn.PubliclyAccessible = &public
	dbIn.SetMasterUserPassword("changeme")
	dbIn.SetMasterUsername("root")


	if c.Type == CloudDBTypeProd{
		// change instance type and setup backup
	}
	resp, err := rdsApi.CreateDBInstance(dbIn)
	if err != nil{
		return nil, errors.Wrap(err, "failed to create rds instance ")
	}
	fmt.Println("waiting for db to be ready")
	if err := rdsApi.WaitUntilDBInstanceAvailable(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier:dbIn.DBInstanceIdentifier,
	}); err != nil{
		return nil, errors.Wrap(err, "failed to wait for db instance to be ready")
	}

	coords := &CloudDBCoords{}
	if resp != nil && resp.DBInstance.DbInstancePort != nil && resp.DBInstance.Endpoint != nil{
			coords.Host = resp.DBInstance.Endpoint.String()
			coords.Port = *resp.DBInstance.DbInstancePort
	}
	coords.Password = *dbIn.MasterUserPassword
	coords.Username = *dbIn.MasterUsername
	return coords, nil
}

func (awsServ *AWSServices)RemoveCloudDB(c CloudDB)error   {
	fmt.Println("delete cloud db ")
	rdsApi := rds.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	deleteDbInput := &rds.DeleteDBInstanceInput{}
	reg := regexp.MustCompile("\\s+")
	c.ClusterName = reg.ReplaceAllString(c.ClusterName, "-")
	deleteDbInput.DBInstanceIdentifier = &c.ClusterName
	deleteBackups := true
	deleteDbInput.DeleteAutomatedBackups = &deleteBackups
	skipFinalSnap := true
	deleteDbInput.SkipFinalSnapshot = &skipFinalSnap

	if _, err := rdsApi.DeleteDBInstance(deleteDbInput); err != nil{
		return errors.Wrap(err, "failed to delete db instance " + c.ClusterName)
	}
	if err := rdsApi.WaitUntilDBInstanceDeleted(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier:deleteDbInput.DBInstanceIdentifier,
	}); err != nil{
		return errors.Wrap(err, "failed to wait for instance to delete " + c.ClusterName)
	}
	return nil
}

func (awsServ *AWSServices)RemoveCloudCache(c CloudCache)error  {
	cacheApi := elasticache.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	reg := regexp.MustCompile("\\s+")
	cci := &elasticache.DeleteCacheClusterInput{}
	cci.SetCacheClusterId(reg.ReplaceAllString(c.ClusterName, "-"))
	if _, err := cacheApi.DeleteCacheCluster(cci); err != nil{
		return errors.Wrap(err, "failed to remove elasicache cluster")
	}

	return nil
}

func (awsServ *AWSServices)CreateCloudCache(c CloudCache)(*CloudCacheCoords,error)  {
	cacheApi := elasticache.New(awsServ.awsSession, aws.NewConfig().WithRegion(region))
	reg := regexp.MustCompile("\\s+")
	cci := &elasticache.CreateCacheClusterInput{}
	cci.SetCacheClusterId(reg.ReplaceAllString(c.ClusterName, "-"))
	cci.SetAuthToken("changeme")
	cci.SetAutoMinorVersionUpgrade(true)
	cci.SetCacheNodeType("cache.t1.micro")
	cci.SetEngine("redis")
	cci.SetEngineVersion(c.EngineVersion)
	cci.SetNumCacheNodes(1)
	cci.SetPort(6397)
	//cci.SetPreferredAvailabilityZone()
	cci.SetSnapshotRetentionLimit(2)
	if _ , err := cacheApi.CreateCacheCluster(cci); err != nil{
		return nil,err
	}
	return &CloudCacheCoords{}, nil
}