locals {
  name = "${var.project_name}-${var.environment}-auth"

  tags = {
    Project     = var.project_name
    Environment = var.environment
    Service     = "auth"
    ManagedBy   = "terraform"
  }

  new_relic_enabled = var.new_relic_license_key != ""

  # Sem URL explícita, usa o NLB que o Kubernetes criou para o Service os-service-public.
  os_service_url = var.os_service_url != "" ? var.os_service_url : "http://${data.aws_lb.os_service[0].dns_name}"

  new_relic_env = local.new_relic_enabled ? {
    NEW_RELIC_ACCOUNT_ID                   = var.new_relic_account_id
    NEW_RELIC_TRUSTED_ACCOUNT_KEY          = var.new_relic_account_id
    NEW_RELIC_LICENSE_KEY                  = var.new_relic_license_key
    NEW_RELIC_LAMBDA_EXTENSION_ENABLED     = "true"
    NEW_RELIC_EXTENSION_SEND_FUNCTION_LOGS = "true"
    NEW_RELIC_APM_LAMBDA_MODE              = "true"
    NR_TAGS                                = "NR.Apm.Lambda.Mode:true;Service:auth;Environment:${var.environment}"
  } : {}
}

# O controlador de nuvem do Kubernetes marca o NLB com o namespace/nome do Service.
data "aws_lb" "os_service" {
  count = var.os_service_url == "" ? 1 : 0

  tags = {
    "kubernetes.io/service-name" = "${var.k8s_namespace}/os-service-public"
  }
}

# Sem vpc_config: desde a Fase 4 a Lambda não abre mais o banco (consulta o os-service pela
# rota interna). Fora da VPC ela tem saída para a internet sem NAT, que a extension do
# New Relic precisa, e o cold start não espera a ENI (ADR-0014).

# tfsec:ignore:aws-cloudwatch-log-group-customer-key O Learner Lab não permite criar CMK; o log fica com a chave gerenciada da AWS.
resource "aws_cloudwatch_log_group" "issuer" {
  name              = "/aws/lambda/${local.name}-issuer"
  retention_in_days = var.log_retention_days

  tags = local.tags
}

# tfsec:ignore:aws-cloudwatch-log-group-customer-key O Learner Lab não permite criar CMK; o log fica com a chave gerenciada da AWS.
resource "aws_cloudwatch_log_group" "authorizer" {
  name              = "/aws/lambda/${local.name}-authorizer"
  retention_in_days = var.log_retention_days

  tags = local.tags
}

# tfsec:ignore:aws-lambda-enable-tracing O trace distribuído é do New Relic (nrlambda + extension); o X-Ray duplicaria o custo sem ligar os spans aos serviços.
resource "aws_lambda_function" "issuer" {
  function_name = "${local.name}-issuer"
  description   = "Troca o CPF do cliente por um JWT (POST /auth)"
  role          = data.aws_iam_role.lab.arn

  filename         = var.issuer_artifact_path
  source_code_hash = filebase64sha256(var.issuer_artifact_path)

  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["arm64"]
  timeout       = 10
  memory_size   = 256
  layers        = local.new_relic_enabled ? [var.new_relic_layer_arn] : []

  environment {
    variables = merge({
      OS_SERVICE_URL     = local.os_service_url
      INTERNAL_API_TOKEN = var.internal_api_token
      JWT_SECRET         = var.jwt_secret
      JWT_TTL            = var.jwt_ttl
      UPSTREAM_TIMEOUT   = var.upstream_timeout
      LOG_LEVEL          = "info"
    }, local.new_relic_env)
  }

  depends_on = [aws_cloudwatch_log_group.issuer]

  tags = local.tags
}

# tfsec:ignore:aws-lambda-enable-tracing O trace distribuído é do New Relic (nrlambda + extension); o X-Ray duplicaria o custo sem ligar os spans aos serviços.
resource "aws_lambda_function" "authorizer" {
  function_name = "${local.name}-authorizer"
  description   = "Lambda authorizer do API Gateway (valida o JWT)"
  role          = data.aws_iam_role.lab.arn

  filename         = var.authorizer_artifact_path
  source_code_hash = filebase64sha256(var.authorizer_artifact_path)

  runtime       = "provided.al2023"
  handler       = "bootstrap"
  architectures = ["arm64"]
  timeout       = 5
  memory_size   = 128
  layers        = local.new_relic_enabled ? [var.new_relic_layer_arn] : []

  environment {
    variables = merge({
      JWT_SECRET = var.jwt_secret
      LOG_LEVEL  = "info"
    }, local.new_relic_env)
  }

  depends_on = [aws_cloudwatch_log_group.authorizer]

  tags = local.tags
}
