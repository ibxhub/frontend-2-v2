variable "app_name" {
  description = "Application name; used as prefix for bucket and secret names."
  type        = string
  default     = "frontend-2-v2"
}

variable "environment" {
  description = "Deployment environment (dev, stage, prod)."
  type        = string
  default     = "dev"

  validation {
    condition     = contains(["dev", "stage", "prod"], var.environment)
    error_message = "environment must be one of: dev, stage, prod."
  }
}

variable "aws_region" {
  description = "AWS region for all resources."
  type        = string
  default     = "us-east-1"
}

variable "owner" {
  description = "Owning team or individual email for tagging."
  type        = string
  default     = "snayak@infoblox.com"
}

variable "bucket_name" {
  description = "Optional override for the uploads bucket name. Leave empty to derive from app_name + environment + account."
  type        = string
  default     = ""
}
