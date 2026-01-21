variable "account_map_enabled" {
  type        = bool
  description = "Set to true to use the account-map component for account lookups. Set to false to use the static account_map variable."
  default     = true
}

variable "account_map" {
  type        = any
  description = "Account map to use when account_map_enabled is false. Expected to contain at least 'full_account_map' with account name to ID mappings."
  default     = {}
}

provider "aws" {
  region = var.region

  # Profile is deprecated in favor of terraform_role_arn. When profiles are not in use, terraform_profile_name is null.
  profile = module.iam_roles.terraform_profile_name

  dynamic "assume_role" {
    # module.iam_roles.terraform_role_arn may be null, in which case do not assume a role.
    for_each = compact([module.iam_roles.terraform_role_arn])
    content {
      role_arn = assume_role.value
    }
  }
}

module "iam_roles" {
  source  = "../../account-map/modules/iam-roles"
  context = module.this.context
}
