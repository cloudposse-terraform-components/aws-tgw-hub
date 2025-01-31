

<!-- markdownlint-disable -->
<a href="https://cpco.io/homepage"><img src="https://github.com/cloudposse-terraform-components/aws-tgw-hub/blob/main/.github/banner.png?raw=true" alt="Project Banner"/></a><br/>
    <p align="right">
<a href="https://github.com/cloudposse-terraform-components/aws-tgw-hub/releases/latest"><img src="https://img.shields.io/github/release/cloudposse-terraform-components/aws-tgw-hub.svg?style=for-the-badge" alt="Latest Release"/></a><a href="https://slack.cloudposse.com"><img src="https://slack.cloudposse.com/for-the-badge.svg" alt="Slack Community"/></a></p>
<!-- markdownlint-restore -->

<!--




  ** DO NOT EDIT THIS FILE
  **
  ** This file was automatically generated by the `cloudposse/build-harness`.
  ** 1) Make all changes to `README.yaml`
  ** 2) Run `make init` (you only need to do this once)
  ** 3) Run`make readme` to rebuild this file.
  **
  ** (We maintain HUNDREDS of open source projects. This is how we maintain our sanity.)
  **





-->

This component is responsible for provisioning an [AWS Transit Gateway](https://aws.amazon.com/transit-gateway) `hub`
that acts as a centralized gateway for connecting VPCs from other `spoke` accounts.


> [!TIP]
> #### 👽 Use Atmos with Terraform
> Cloud Posse uses [`atmos`](https://atmos.tools) to easily orchestrate multiple environments using Terraform. <br/>
> Works with [Github Actions](https://atmos.tools/integrations/github-actions/), [Atlantis](https://atmos.tools/integrations/atlantis), or [Spacelift](https://atmos.tools/integrations/spacelift).
>
> <details>
> <summary><strong>Watch demo of using Atmos with Terraform</strong></summary>
> <img src="https://github.com/cloudposse/atmos/blob/main/docs/demo.gif?raw=true"/><br/>
> <i>Example of running <a href="https://atmos.tools"><code>atmos</code></a> to manage infrastructure from our <a href="https://atmos.tools/quick-start/">Quick Start</a> tutorial.</i>
> </detalis>





## Usage


**Stack Level**: Regional

## Basic Usage with `tgw/spoke`

Here's an example snippet for how to configure and use this component:

```yaml
components:
  terraform:
    tgw/hub/defaults:
      metadata:
        type: abstract
        component: tgw/hub
      vars:
        enabled: true
        name: tgw-hub
        expose_eks_sg: false
        tags:
          Team: sre
          Service: tgw-hub

    tgw/hub:
      metadata:
        inherits:
          - tgw/hub/defaults
        component: tgw/hub
      vars:
        connections:
          - account:
              tenant: core
              stage: network
            vpc_component_names:
              - vpc-dev
          - account:
              tenant: core
              stage: artifacts
          - account:
              tenant: core
              stage: auto
            eks_component_names:
              - eks/cluster
          - account:
              tenant: plat
              stage: dev
            vpc_component_names:
              - vpc
              - vpc/data/1
            eks_component_names:
              - eks/cluster
          - account:
              tenant: plat
              stage: staging
            vpc_component_names:
              - vpc
              - vpc/data/1
            eks_component_names:
              - eks/cluster
          - account:
              tenant: plat
              stage: prod
            vpc_component_names:
              - vpc
              - vpc/data/1
            eks_component_names:
              - eks/cluster
```

To provision the Transit Gateway and all related resources, run the following commands:

```sh
atmos terraform plan tgw/hub -s <tenant>-<environment>-network
atmos terraform apply tgw/hub -s <tenant>-<environment>-network
```

## Alternate Usage with `tgw/attachment`, `tgw/routes`, and `vpc/routes`

### Components Overview

- **`tgw/hub`**: Creates the Transit Gateway in the network account
- **`tgw/attachment`**: Creates and manages Transit Gateway VPC attachments in connected accounts
- **`tgw/hub-connection`**: Creates the Transit Gateway peering connection between two `tgw/hub` deployments
- **`tgw/routes`**: Manages Transit Gateway route tables in the network account
- **`vpc-routes`** (`vpc/routes/private`): Configures VPC route tables in connected accounts to route traffic through the Transit Gateway (Note: This component lives outside the `tgw/` directory since it's not specific to Transit Gateway)

### Architecture

The Transit Gateway components work together in the following way:

1. Transit Gateway is created in the network account (`tgw/hub`)
2. VPCs in other accounts attach to the Transit Gateway (`tgw/attachment`)
3. Route tables in connected VPCs direct traffic across accounts (`vpc-routes`)
4. Transit Gateway route tables control routing between attachments (`tgw/routes`)

```mermaid
graph TD
    subgraph core-use1-network
        TGW[Transit Gateway]
        TGW_RT[TGW Route Tables]
    end

    subgraph plat-use1-dev
        VPC1[VPC]
        VPC1_RT[VPC Route Tables]
        ATT1[TGW Attachment]
    end

    ATT1 <--> TGW
    ATT2 <--> TGW
    TGW <--> TGW_RT
    VPC1_RT <--> VPC1
    VPC2_RT <--> VPC2
    VPC1 <--> ATT1
    VPC2 <--> ATT2
```

### Deployment Steps

#### 1. Deploy Transit Gateway Hub

First, create the Transit Gateway in the network account:

```yaml
components:
  terraform:
    tgw/hub:
      vars:
        connections: []
```

#### 2. Deploy VPC Attachments

Important: Deploy attachments in connected accounts first, before deploying attachments in the network account.

##### Connected Account Attachments

```yaml
components:
  terraform:
    tgw/attachment:
      vars:
        transit_gateway_id: !terraform.output tgw/hub core-use1-network transit_gateway_id
        transit_gateway_route_table_id: !terraform.output tgw/hub core-use1-network transit_gateway_route_table_id
        create_transit_gateway_route_table_association: false
```

##### Network Account Attachment

```yaml
components:
  terraform:
    tgw/attachment:
      vars:
        transit_gateway_id: !terraform.output tgw/hub core-use1-network transit_gateway_id
        transit_gateway_route_table_id: !terraform.output tgw/hub core-use1-network transit_gateway_route_table_id

        # Route table associations are required so that route tables can propagate their routes to other route tables.
        # Set the following to true in the same account where the Transit Gateway and its route tables are deployed
        create_transit_gateway_route_table_association: true
```

#### 3. Configure VPC Routes

Configure routes in all connected VPCs.

```yaml
components:
  terraform:
    vpc/routes/private:
      metadata:
        component: vpc-routes
      vars:
        route_table_ids: !terraform.output vpc private_route_table_ids
        routes:
          # Route to network account
          - destination:
              cidr_block: !terraform.output vpc core-use1-network vpc_cidr
            target:
              type: transit_gateway_id
              value: !terraform.output tgw/hub core-use1-network transit_gateway_id

          # Route to core-auto account, if necessary
          - destination:
              cidr_block: !terraform.output vpc core-use1-auto vpc_cidr
            target:
              type: transit_gateway_id
              value: !terraform.output tgw/hub core-use1-network transit_gateway_id
```

Configure routes in the Network Account VPCs.

```yaml
components:
  terraform:
    vpc/routes/private:
      vars:
        route_table_ids: !terraform.output vpc private_route_table_ids
        routes:
          # Routes to connected accounts
          - destination:
              cidr_block: !terraform.output vpc core-use1-auto vpc_cidr
            target:
              type: transit_gateway_id
              value: !terraform.output tgw/hub transit_gateway_id
          - destination:
              cidr_block: !terraform.output vpc plat-use1-dev vpc_cidr
            target:
              type: transit_gateway_id
              value: !terraform.output tgw/hub transit_gateway_id
```

### 4. Deploy Transit Gateway Route Table Routes

Deploy the `tgw/routes` component in the network account to create route tables and routes.

```yaml
components:
  terraform:
    tgw/routes:
      vars:
        transit_gateway_route_table_id: !terraform.output tgw/hub transit_gateway_route_table_id
        # Use propagated routes to route through VPC attachments
        propagated_routes:
          # Route to this account
          - attachment_id: !terraform.output tgw/attachment core-use1-network transit_gateway_attachment_id
          # Route to any connected account
          - attachment_id: !terraform.output tgw/attachment core-use1-auto transit_gateway_attachment_id
          - attachment_id: !terraform.output tgw/attachment plat-use1-dev transit_gateway_attachment_id
```

> [!IMPORTANT]
> In Cloud Posse's examples, we avoid pinning modules to specific versions to prevent discrepancies between the documentation
> and the latest released versions. However, for your own projects, we strongly advise pinning each module to the exact version
> you're using. This practice ensures the stability of your infrastructure. Additionally, we recommend implementing a systematic
> approach for updating versions to avoid unexpected changes.









## Related Projects

Check out these related projects.

- [Cloud Posse Terraform Modules](https://docs.cloudposse.com/modules/) - Our collection of reusable Terraform modules used by our reference architectures.
- [Atmos](https://atmos.tools) - Atmos is like docker-compose but for your infrastructure


> [!TIP]
> #### Use Terraform Reference Architectures for AWS
>
> Use Cloud Posse's ready-to-go [terraform architecture blueprints](https://cloudposse.com/reference-architecture/) for AWS to get up and running quickly.
>
> ✅ We build it together with your team.<br/>
> ✅ Your team owns everything.<br/>
> ✅ 100% Open Source and backed by fanatical support.<br/>
>
> <a href="https://cpco.io/commercial-support?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=commercial_support"><img alt="Request Quote" src="https://img.shields.io/badge/request%20quote-success.svg?style=for-the-badge"/></a>
> <details><summary>📚 <strong>Learn More</strong></summary>
>
> <br/>
>
> Cloud Posse is the leading [**DevOps Accelerator**](https://cpco.io/commercial-support?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=commercial_support) for funded startups and enterprises.
>
> *Your team can operate like a pro today.*
>
> Ensure that your team succeeds by using Cloud Posse's proven process and turnkey blueprints. Plus, we stick around until you succeed.
> #### Day-0:  Your Foundation for Success
> - **Reference Architecture.** You'll get everything you need from the ground up built using 100% infrastructure as code.
> - **Deployment Strategy.** Adopt a proven deployment strategy with GitHub Actions, enabling automated, repeatable, and reliable software releases.
> - **Site Reliability Engineering.** Gain total visibility into your applications and services with Datadog, ensuring high availability and performance.
> - **Security Baseline.** Establish a secure environment from the start, with built-in governance, accountability, and comprehensive audit logs, safeguarding your operations.
> - **GitOps.** Empower your team to manage infrastructure changes confidently and efficiently through Pull Requests, leveraging the full power of GitHub Actions.
>
> <a href="https://cpco.io/commercial-support?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=commercial_support"><img alt="Request Quote" src="https://img.shields.io/badge/request%20quote-success.svg?style=for-the-badge"/></a>
>
> #### Day-2: Your Operational Mastery
> - **Training.** Equip your team with the knowledge and skills to confidently manage the infrastructure, ensuring long-term success and self-sufficiency.
> - **Support.** Benefit from a seamless communication over Slack with our experts, ensuring you have the support you need, whenever you need it.
> - **Troubleshooting.** Access expert assistance to quickly resolve any operational challenges, minimizing downtime and maintaining business continuity.
> - **Code Reviews.** Enhance your team’s code quality with our expert feedback, fostering continuous improvement and collaboration.
> - **Bug Fixes.** Rely on our team to troubleshoot and resolve any issues, ensuring your systems run smoothly.
> - **Migration Assistance.** Accelerate your migration process with our dedicated support, minimizing disruption and speeding up time-to-value.
> - **Customer Workshops.** Engage with our team in weekly workshops, gaining insights and strategies to continuously improve and innovate.
>
> <a href="https://cpco.io/commercial-support?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=commercial_support"><img alt="Request Quote" src="https://img.shields.io/badge/request%20quote-success.svg?style=for-the-badge"/></a>
> </details>

## ✨ Contributing

This project is under active development, and we encourage contributions from our community.



Many thanks to our outstanding contributors:

<a href="https://github.com/cloudposse-terraform-components/aws-tgw-hub/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=cloudposse-terraform-components/aws-tgw-hub&max=24" />
</a>

For 🐛 bug reports & feature requests, please use the [issue tracker](https://github.com/cloudposse-terraform-components/aws-tgw-hub/issues).

In general, PRs are welcome. We follow the typical "fork-and-pull" Git workflow.
 1. Review our [Code of Conduct](https://github.com/cloudposse-terraform-components/aws-tgw-hub/?tab=coc-ov-file#code-of-conduct) and [Contributor Guidelines](https://github.com/cloudposse/.github/blob/main/CONTRIBUTING.md).
 2. **Fork** the repo on GitHub
 3. **Clone** the project to your own machine
 4. **Commit** changes to your own branch
 5. **Push** your work back up to your fork
 6. Submit a **Pull Request** so that we can review your changes

**NOTE:** Be sure to merge the latest changes from "upstream" before making a pull request!

### 🌎 Slack Community

Join our [Open Source Community](https://cpco.io/slack?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=slack) on Slack. It's **FREE** for everyone! Our "SweetOps" community is where you get to talk with others who share a similar vision for how to rollout and manage infrastructure. This is the best place to talk shop, ask questions, solicit feedback, and work together as a community to build totally *sweet* infrastructure.

### 📰 Newsletter

Sign up for [our newsletter](https://cpco.io/newsletter?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=newsletter) and join 3,000+ DevOps engineers, CTOs, and founders who get insider access to the latest DevOps trends, so you can always stay in the know.
Dropped straight into your Inbox every week — and usually a 5-minute read.

### 📆 Office Hours <a href="https://cloudposse.com/office-hours?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=office_hours"><img src="https://img.cloudposse.com/fit-in/200x200/https://cloudposse.com/wp-content/uploads/2019/08/Powered-by-Zoom.png" align="right" /></a>

[Join us every Wednesday via Zoom](https://cloudposse.com/office-hours?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=office_hours) for your weekly dose of insider DevOps trends, AWS news and Terraform insights, all sourced from our SweetOps community, plus a _live Q&A_ that you can’t find anywhere else.
It's **FREE** for everyone!
## License

<a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=for-the-badge" alt="License"></a>

<details>
<summary>Preamble to the Apache License, Version 2.0</summary>
<br/>
<br/>



```text
Licensed to the Apache Software Foundation (ASF) under one
or more contributor license agreements.  See the NOTICE file
distributed with this work for additional information
regarding copyright ownership.  The ASF licenses this file
to you under the Apache License, Version 2.0 (the
"License"); you may not use this file except in compliance
with the License.  You may obtain a copy of the License at

  https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing,
software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
KIND, either express or implied.  See the License for the
specific language governing permissions and limitations
under the License.
```
</details>

## Trademarks

All other trademarks referenced herein are the property of their respective owners.


---
Copyright © 2017-2025 [Cloud Posse, LLC](https://cpco.io/copyright)


<a href="https://cloudposse.com/readme/footer/link?utm_source=github&utm_medium=readme&utm_campaign=cloudposse-terraform-components/aws-tgw-hub&utm_content=readme_footer_link"><img alt="README footer" src="https://cloudposse.com/readme/footer/img"/></a>

<img alt="Beacon" width="0" src="https://ga-beacon.cloudposse.com/UA-76589703-4/cloudposse-terraform-components/aws-tgw-hub?pixel&cs=github&cm=readme&an=aws-tgw-hub"/>
