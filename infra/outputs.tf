output "issuer_function_name" {
  value = aws_lambda_function.issuer.function_name
}

output "issuer_invoke_arn" {
  value       = aws_lambda_function.issuer.invoke_arn
  description = "Valor de lambda_issuer_invoke_arn no postech-tc4-infra-k8s"
}

output "authorizer_function_name" {
  value = aws_lambda_function.authorizer.function_name
}

output "authorizer_invoke_arn" {
  value       = aws_lambda_function.authorizer.invoke_arn
  description = "Valor de lambda_authorizer_invoke_arn no postech-tc4-infra-k8s"
}

output "log_groups" {
  value = [aws_cloudwatch_log_group.issuer.name, aws_cloudwatch_log_group.authorizer.name]
}

output "os_service_url" {
  value       = local.os_service_url
  description = "Endereço do os-service que o issuer consulta"
}
