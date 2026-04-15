output "puzzle_table_name" {
  description = "DynamoDB puzzle pool table name"
  value       = aws_dynamodb_table.puzzle_pool.name
}

output "puzzle_table_arn" {
  description = "DynamoDB puzzle pool table ARN"
  value       = aws_dynamodb_table.puzzle_pool.arn
}
