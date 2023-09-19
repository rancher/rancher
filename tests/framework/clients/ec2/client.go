package ec2

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/rancher/rancher/tests/framework/pkg/config"
)

// Client is a struct that wraps the needed AWSEC2Config object, and ec2.EC2 which makes the actual calls to aws.
type Client struct {
	SVC          *ec2.EC2
	ClientConfig *AWSEC2Configs
}

// NewClient is a constructor that creates an *Client which a wrapper for a "github.com/aws/aws-sdk-go/service/ec2" session and
// the aws ec2 config.
func NewClient() (*Client, error) {
	awsEC2ClientConfig := new(AWSEC2Configs)

	config.LoadConfig(ConfigurationFileKey, awsEC2ClientConfig)

	credential := credentials.NewStaticCredentials(awsEC2ClientConfig.AWSAccessKeyID, awsEC2ClientConfig.AWSSecretAccessKey, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: credential,
		Region:      aws.String(awsEC2ClientConfig.Region)},
	)
	if err != nil {
		return nil, err
	}

	// Create EC2 service client
	svc := ec2.New(sess)
	return &Client{
		SVC:          svc,
		ClientConfig: awsEC2ClientConfig,
	}, nil
}
