variable "aws_region" {
  type    = string
  default = "us-east-1"
}

variable "project_name" {
  type    = string
  default = "postech-tc4"
}

variable "environment" {
  type = string
}

variable "os_service_url" {
  type        = string
  default     = ""
  description = "URL base do os-service; vazio = procura o NLB do Service os-service-public no namespace"

  validation {
    condition     = var.os_service_url == "" || can(regex("^https?://", var.os_service_url))
    error_message = "os_service_url precisa começar com http:// ou https://."
  }
}

variable "k8s_namespace" {
  type        = string
  default     = "postech"
  description = "Namespace do os-service no EKS"
}

variable "internal_api_token" {
  type        = string
  sensitive   = true
  description = "Mesmo valor de INTERNAL_API_TOKEN no os-service"
}

variable "jwt_secret" {
  type      = string
  sensitive = true
}

variable "jwt_ttl" {
  type    = string
  default = "15m"
}

variable "upstream_timeout" {
  type    = string
  default = "3s"
}

variable "issuer_artifact_path" {
  type    = string
  default = "../issuer.zip"
}

variable "authorizer_artifact_path" {
  type    = string
  default = "../authorizer.zip"
}

variable "log_retention_days" {
  type    = number
  default = 14
}

variable "lab_role_name" {
  type        = string
  default     = "LabRole"
  description = "No Learner Lab não dá para criar IAM: as functions usam a LabRole"
}

variable "new_relic_account_id" {
  type    = string
  default = ""
}

variable "new_relic_license_key" {
  type      = string
  sensitive = true
  default   = ""
}

variable "new_relic_layer_arn" {
  type        = string
  default     = "arn:aws:lambda:us-east-1:451483290750:layer:NewRelicLambdaExtensionARM64:62"
  description = "Extension do New Relic para arm64 (provided.al2023)"
}
