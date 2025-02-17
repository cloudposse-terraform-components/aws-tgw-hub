package test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/cloudposse/test-helpers/pkg/atmos"
	helper "github.com/cloudposse/test-helpers/pkg/atmos/component-helper"
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

type ComponentSuite struct {
	helper.TestSuite
}

func (s *ComponentSuite) TestBasic() {
	const component = "tgw-hub/basic"
	const stack = "default-test"
	const awsRegion = "us-east-2"

	defer s.DestroyAtmosComponent(s.T(), component, stack, nil)
	options, _ := s.DeployAtmosComponent(s.T(), component, stack, nil)
	assert.NotNil(s.T(), options)

	transitGatewayArn := atmos.Output(s.T(), options, "transit_gateway_arn")
	assert.Empty(s.T(), transitGatewayArn)

	transitGatewayId := atmos.Output(s.T(), options, "transit_gateway_id")
	assert.NotEmpty(s.T(), transitGatewayId)

	transitGatewayRouteTableId := atmos.Output(s.T(), options, "transit_gateway_route_table_id")
	assert.NotEmpty(s.T(), transitGatewayRouteTableId)

	var vpcs map[string]VPCOutput
	atmos.OutputStruct(s.T(), options, "vpcs", &vpcs)

	vpc := vpcs["default-ue2-test-vpc"]
	assert.Equal(s.T(), "local", vpc.BackendType)
	assert.Nil(s.T(), vpc.RemoteWorkspaceName)
	assert.Nil(s.T(), vpc.S3WorkspaceName)
	assert.Equal(s.T(), "default-test", vpc.WorkspaceName)

	assert.Equal(s.T(), "ue2", vpc.Outputs.Environment)
	assert.Equal(s.T(), "default", vpc.Outputs.Tenant)
	assert.Equal(s.T(), "test", vpc.Outputs.Stage)
	assert.NotEmpty(s.T(), vpc.Outputs.VPC.ID)
	assert.Equal(s.T(), "172.16.0.0/16", vpc.Outputs.VPC.CIDR)
	assert.Equal(s.T(), "eg.cptest.co/subnet/type", vpc.Outputs.VPC.SubnetTypeTagKey)

	// Additional VPC outputs asserts
	assert.NotEmpty(s.T(), vpc.Outputs.PrivateRouteTableIDs)
	assert.NotEmpty(s.T(), vpc.Outputs.PrivateSubnetCIDRs)
	assert.NotEmpty(s.T(), vpc.Outputs.PublicRouteTableIDs)
	assert.NotEmpty(s.T(), vpc.Outputs.PublicSubnetCIDRs)
	assert.NotEmpty(s.T(), vpc.Outputs.RouteTables)
	assert.NotEmpty(s.T(), vpc.Outputs.Subnets)

	eks := atmos.OutputMapOfObjects(s.T(), options, "eks")
	assert.Empty(s.T(), eks)

	var tgwConfig TGWOutput
	atmos.OutputStruct(s.T(), options, "tgw_config", &tgwConfig)
	assert.NotNil(s.T(), tgwConfig)
	assert.Equal(s.T(), transitGatewayId, tgwConfig.ExistingTransitGatewayID)
	assert.NotEmpty(s.T(), tgwConfig.ExistingTransitGatewayRouteTableID)
	assert.False(s.T(), tgwConfig.ExposeEKS_SG)
	assert.Equal(s.T(), vpcs, tgwConfig.VPCs)

	client := aws.NewEc2Client(s.T(), awsRegion)
	transitGatewayOutput, err := client.DescribeTransitGateways(context.Background(), &ec2.DescribeTransitGatewaysInput{
		TransitGatewayIds: []string{transitGatewayId},
	})
	assert.NoError(s.T(), err)
	transitGateway := transitGatewayOutput.TransitGateways[0]
	assert.Equal(s.T(), 1, len(transitGatewayOutput.TransitGateways))
	assert.EqualValues(s.T(), "available", transitGateway.State)

	routeTableOutput, err := client.DescribeTransitGatewayRouteTables(context.Background(), &ec2.DescribeTransitGatewayRouteTablesInput{
		TransitGatewayRouteTableIds: []string{transitGatewayRouteTableId},
	})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, len(routeTableOutput.TransitGatewayRouteTables))
	routeTable := routeTableOutput.TransitGatewayRouteTables[0]
	assert.Equal(s.T(), transitGatewayRouteTableId, *routeTable.TransitGatewayRouteTableId)
	assert.EqualValues(s.T(), "available", routeTable.State)
	assert.False(s.T(), *routeTable.DefaultAssociationRouteTable)
	assert.False(s.T(), *routeTable.DefaultPropagationRouteTable)
}

func TestRunSuite(t *testing.T) {
	suite := new(ComponentSuite)

	suite.AddDependency(t, "vpc", "default-test", nil)
	helper.Run(t, suite)
}
