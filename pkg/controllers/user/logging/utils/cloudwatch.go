package utils

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/pkg/errors"
	v3 "github.com/rancher/types/apis/management.cattle.io/v3"
	"github.com/rancher/types/config/dialer"
)

const testStream = "__rancher_test"

type cloudwatchTestWrap struct {
	*v3.CloudWatchConfig
}

func (c *cloudwatchTestWrap) TestReachable(dial dialer.Dialer, includeSendTestLog bool) error {
	cwl := cloudwatchlogs.New(
		session.Must(
			session.NewSession(
				aws.NewConfig().
					WithCredentials(credentials.NewStaticCredentials(c.AccessKeyID, c.SecretAccessKey, "")).
					WithRegion(c.Region),
			),
		),
	)

	result, err := cwl.DescribeLogGroups(&cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(c.Group),
	})

	if err != nil {
		return errors.Wrapf(err, "failed to check log group %s", c.Group)
	}

	for _, g := range result.LogGroups {
		if aws.StringValue(g.LogGroupName) == c.Group {
			if includeSendTestLog {
				return c.checkGroup(cwl)
			}

			return nil
		}
	}

	return fmt.Errorf("log group '%s' not found in %s", c.Group, c.Region)
}

func (c *cloudwatchTestWrap) checkGroup(cwl *cloudwatchlogs.CloudWatchLogs) error {
	st, err := c.createTestStreamIfNotExists(cwl)
	if err != nil {
		return err
	}

	_, err = cwl.PutLogEvents(&cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(c.Group),
		LogStreamName: aws.String(testStream),
		SequenceToken: st,
		LogEvents: []*cloudwatchlogs.InputLogEvent{
			{Timestamp: aws.Int64(time.Now().UTC().Unix()), Message: aws.String("Test Event from Rancher")},
		},
	})

	if err != nil {
		return errors.Wrapf(err, "failed to put test event into stream")
	}

	return nil
}

func (c *cloudwatchTestWrap) createTestStreamIfNotExists(cwl *cloudwatchlogs.CloudWatchLogs) (*string, error) {
	result, err := cwl.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName:        aws.String(c.Group),
		LogStreamNamePrefix: aws.String(testStream),
	})

	if err != nil {
		return nil, errors.Wrapf(err, "Unable to check for test stream %s in group %s", testStream, c.Group)
	}

	for _, s := range result.LogStreams {
		if aws.StringValue(s.LogStreamName) == testStream {
			return s.UploadSequenceToken, nil
		}
	}

	_, err = cwl.CreateLogStream(&cloudwatchlogs.CreateLogStreamInput{
		LogGroupName:  aws.String(c.Group),
		LogStreamName: aws.String(testStream),
	})

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create test stream in group %s", c.Group)
	}

	return nil, nil
}
