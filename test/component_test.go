package test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/cloudposse/test-helpers/pkg/atmos"
	helper "github.com/cloudposse/test-helpers/pkg/atmos/aws-component-helper"
	"github.com/gruntwork-io/terratest/modules/aws"
	"github.com/stretchr/testify/assert"
)

type VPC struct {
	CIDR             string `json:"cidr"`
	ID               string `json:"id"`
	SubnetTypeTagKey string `json:"subnet_type_tag_key"`
}

type RouteTable struct {
	IDs []string `json:"ids"`
}

type Subnet struct {
	CIDR []string `json:"cidr"`
	IDs  []string `json:"ids"`
}

type VPCOutputOutputs struct {
	AvailabilityZones   []string            `json:"availability_zones"`
	AZPrivateSubnetsMap map[string][]string `json:"az_private_subnets_map"`
	AZPublicSubnetsMap  map[string][]string `json:"az_public_subnets_map"`
	Environment         string              `json:"environment"`
	MaxSubnetCount      int                 `json:"max_subnet_count"`
	// "interface_vpc_endpoints": []interface{}{},
	// "nat_eip_protections": map[string]interface{}{},
	// "nat_gateway_ids": []interface{}{},
	// "nat_gateway_public_ips": []interface{}{},
	// "nat_instance_ids": []interface{}{},
	PrivateRouteTableIDs      []string              `json:"private_route_table_ids"`
	PrivateSubnetCIDRs        []string              `json:"private_subnet_cidrs"`
	PrivateSubnetIDs          []string              `json:"private_subnet_ids"`
	PublicRouteTableIDs       []string              `json:"public_route_table_ids"`
	PublicSubnetCIDRs         []string              `json:"public_subnet_cidrs"`
	PublicSubnetIDs           []string              `json:"public_subnet_ids"`
	RouteTables               map[string]RouteTable `json:"route_tables"`
	Stage                     string                `json:"stage"`
	Subnets                   map[string]Subnet     `json:"subnets"`
	Tenant                    string                `json:"tenant"`
	VPC                       VPC                   `json:"vpc"`
	VPCCIDR                   string                `json:"vpc_cidr"`
	VPCDefaultNetworkACLID    string                `json:"vpc_default_network_acl_id"`
	VPCDefaultSecurityGroupID string                `json:"vpc_default_security_group_id"`
	VPCID                     string                `json:"vpc_id"`
}

type VPCOutput struct {
	Backend             map[string]string `json:"backend"`
	BackendType         string            `json:"backend_type"`
	Outputs             VPCOutputOutputs  `json:"outputs"`
	RemoteWorkspaceName interface{}       `json:"remote_workspace_name"`
	S3WorkspaceName     interface{}       `json:"s3_workspace_name"`
	WorkspaceName       string            `json:"workspace_name"`
}

type TGWOutput struct {
	EKS                                map[string]interface{} `json:"eks"`
	ExistingTransitGatewayID           string                 `json:"existing_transit_gateway_id"`
	ExistingTransitGatewayRouteTableID string                 `json:"existing_transit_gateway_route_table_id"`
	ExposeEKS_SG                       bool                   `json:"expose_eks_sg"`
	VPCs                               map[string]VPCOutput   `json:"vpcs"`
}

func TestComponent(t *testing.T) {
	// Define the AWS region to use for the tests
	awsRegion := "us-east-2"

	// Initialize the test fixture
	fixture := helper.NewFixture(t, "../", awsRegion, "test/fixtures")

	// Ensure teardown is executed after the test
	defer fixture.TearDown()
	fixture.SetUp(&atmos.Options{})

	// Define the test suite
	fixture.Suite("default", func(t *testing.T, suite *helper.Suite) {
		suite.AddDependency("vpc", "default-test")

		// Test phase: Validate the functionality of the ALB component
		suite.Test(t, "basic", func(t *testing.T, atm *helper.Atmos) {
			inputs := map[string]interface{}{}

			defer atm.GetAndDestroy("tgw-hub/basic", "default-test", inputs)
			component := atm.GetAndDeploy("tgw-hub/basic", "default-test", inputs)
			assert.NotNil(t, component)

			transitGatewayArn := atm.Output(component, "transit_gateway_arn")
			assert.Empty(t, "", transitGatewayArn)

			transitGatewayId := atm.Output(component, "transit_gateway_id")
			assert.NotEmpty(t, transitGatewayId)

			transitGatewayRouteTableId := atm.Output(component, "transit_gateway_route_table_id")
			assert.NotEmpty(t, transitGatewayRouteTableId)

			var vpcs map[string]VPCOutput
			atm.OutputStruct(component, "vpcs", &vpcs)

			vpc := vpcs["default-ue2-test-vpc"]
			assert.Equal(t, "local", vpc.BackendType)
			assert.Nil(t, vpc.RemoteWorkspaceName)
			assert.Nil(t, vpc.S3WorkspaceName)
			assert.Equal(t, "default-test", vpc.WorkspaceName)

			assert.Equal(t, "ue2", vpc.Outputs.Environment)
			assert.Equal(t, "default", vpc.Outputs.Tenant)
			assert.Equal(t, "test", vpc.Outputs.Stage)
			assert.NotEmpty(t, vpc.Outputs.VPC.ID)
			assert.Equal(t, "172.16.0.0/16", vpc.Outputs.VPC.CIDR)
			assert.Equal(t, "eg.cptest.co/subnet/type", vpc.Outputs.VPC.SubnetTypeTagKey)

			// Additional VPC outputs asserts
			assert.NotEmpty(t, vpc.Outputs.PrivateRouteTableIDs)
			assert.NotEmpty(t, vpc.Outputs.PrivateSubnetCIDRs)
			assert.NotEmpty(t, vpc.Outputs.PublicRouteTableIDs)
			assert.NotEmpty(t, vpc.Outputs.PublicSubnetCIDRs)
			assert.NotEmpty(t, vpc.Outputs.RouteTables)
			assert.NotEmpty(t, vpc.Outputs.Subnets)

			eks := atm.OutputMapOfObjects(component, "eks")
			assert.Empty(t, eks)

			var tgwConfig TGWOutput
			atm.OutputStruct(component, "tgw_config", &tgwConfig)
			assert.NotNil(t, tgwConfig)
			assert.Equal(t, transitGatewayId, tgwConfig.ExistingTransitGatewayID)
			assert.NotEmpty(t, tgwConfig.ExistingTransitGatewayRouteTableID)
			assert.False(t, tgwConfig.ExposeEKS_SG)
			assert.Equal(t, vpcs, tgwConfig.VPCs)

			client := aws.NewEc2Client(t, awsRegion)
			transitGatewayOutput, err := client.DescribeTransitGateways(context.Background(), &ec2.DescribeTransitGatewaysInput{
				TransitGatewayIds: []string{transitGatewayId},
			})
			assert.NoError(t, err)
			transitGateway := transitGatewayOutput.TransitGateways[0]
			assert.Equal(t, 1, len(transitGatewayOutput.TransitGateways))
			assert.EqualValues(t, "available", transitGateway.State)

			routeTableOutput, err := client.DescribeTransitGatewayRouteTables(context.Background(), &ec2.DescribeTransitGatewayRouteTablesInput{
				TransitGatewayRouteTableIds: []string{transitGatewayRouteTableId},
			})
			assert.NoError(t, err)
			assert.Equal(t, 1, len(routeTableOutput.TransitGatewayRouteTables))
			routeTable := routeTableOutput.TransitGatewayRouteTables[0]
			assert.Equal(t, transitGatewayRouteTableId, *routeTable.TransitGatewayRouteTableId)
			assert.EqualValues(t, "available", routeTable.State)
			assert.False(t, *routeTable.DefaultAssociationRouteTable)
			assert.False(t, *routeTable.DefaultPropagationRouteTable)
		})
	})
}
