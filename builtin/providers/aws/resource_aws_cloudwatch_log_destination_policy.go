package aws

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func resourceAwsCloudWatchLogDestinationPolicy() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsCloudWatchLogDestinationPolicyPut,
		Update: resourceAwsCloudWatchLogDestinationPolicyPut,

		Read:   resourceAwsCloudWatchLogDestinationPolicyRead,
		Delete: resourceAwsCloudWatchLogDestinationPolicyDelete,

		Schema: map[string]*schema.Schema{
			"destination_name": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},

			"access_policy": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceAwsCloudWatchLogDestinationPolicyPut(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).cloudwatchlogsconn

	destination_name := d.Get("destination_name").(string)
	access_policy := d.Get("access_policy").(string)

	params := &cloudwatchlogs.PutDestinationPolicyInput{
		DestinationName: aws.String(destination_name),
		AccessPolicy:    aws.String(access_policy),
	}

	_, err := conn.PutDestinationPolicy(params)

	if err != nil {
		return fmt.Errorf("Error creating DestinationPolicy with destination_name %s: %#v", destination_name, err)
	}

	d.SetId(destination_name)
	return resourceAwsCloudWatchLogDestinationPolicyRead(d, meta)
}

func resourceAwsCloudWatchLogDestinationPolicyRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AWSClient).cloudwatchlogsconn

	destination_name := d.Get("destination_name").(string)

	params := &cloudwatchlogs.DescribeDestinationsInput{
		DestinationNamePrefix: aws.String(destination_name),
	}

	resp, err := conn.DescribeDestinations(params)
	if err != nil {
		return fmt.Errorf("Error reading Destinations with name prefix %s: %#v", destination_name, err)
	}

	for _, destination := range resp.Destinations {
		if *destination.DestinationName == destination_name {
			if destination.AccessPolicy != nil {
				d.Set("access_policy", *destination.AccessPolicy)
			}
			d.SetId(destination_name)
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceAwsCloudWatchLogDestinationPolicyDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
