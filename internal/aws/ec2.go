package aws

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EC2Instance represents basic information about an EC2 instance
type EC2Instance struct {
	InstanceID string
	Name       string
	State      string
	PrivateIP  string
	PublicIP   string
}

// ListRunningInstances returns a list of running EC2 instances in the specified profile and region
func ListRunningInstances(profile, region string) ([]EC2Instance, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		},
	}

	var instances []EC2Instance
	paginator := ec2.NewDescribeInstancesPaginator(client, input)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				name := ""
				for _, tag := range instance.Tags {
					if *tag.Key == "Name" {
						name = *tag.Value
						break
					}
				}

				instances = append(instances, EC2Instance{
					InstanceID: *instance.InstanceId,
					Name:       name,
					State:      string(instance.State.Name),
					PrivateIP:  aws.ToString(instance.PrivateIpAddress),
					PublicIP:   aws.ToString(instance.PublicIpAddress),
				})
			}
		}
	}

	// Sort by name, then by instance ID
	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Name != instances[j].Name {
			return instances[i].Name < instances[j].Name
		}
		return instances[i].InstanceID < instances[j].InstanceID
	})

	return instances, nil
}

// GetInstanceName returns the name of an instance given its ID
func GetInstanceName(profile, region, instanceID string) (string, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), opts...)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := ec2.NewFromConfig(cfg)

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	result, err := client.DescribeInstances(context.TODO(), input)
	if err != nil {
		return "", fmt.Errorf("failed to describe instance %s: %w", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := result.Reservations[0].Instances[0]
	for _, tag := range instance.Tags {
		if *tag.Key == "Name" {
			return *tag.Value, nil
		}
	}

	return "", nil
}
