package ec2

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

type EC2Client struct {
	SVC    *ec2.EC2
	Config *AWSEC2Config
}

// NewEC2Client is a constructor that creates an *EC2Client which a wrapper for an "github.com/aws/aws-sdk-go/service/ec2" session and
// the aws ec2 confit.
func NewEC2Client() (*EC2Client, error) {
	awsEC2Config := new(AWSEC2Config)

	config.LoadConfig(ConfigurationFileKey, awsEC2Config)

	credential := credentials.NewStaticCredentials(awsEC2Config.AWSAccessKeyID, awsEC2Config.AWSSecretAccessKey, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: credential,
		Region:      aws.String(awsEC2Config.Region)},
	)
	if err != nil {
		return nil, err
	}

	// Create EC2 service client
	svc := ec2.New(sess)
	return &EC2Client{
		SVC:    svc,
		Config: awsEC2Config,
	}, nil
}
