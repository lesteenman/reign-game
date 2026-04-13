# Spec: Terraform Infrastructure

Covers R-004.

## Requirements

### TF-01: State Backend

- `infra/backend.tf` configures an S3 backend
- Bucket name, key, region, and DynamoDB lock table name are variables or partial config (not hardcoded)
- State is stored remotely in the pre-existing S3 bucket

### TF-02: Frontend Module

- `infra/modules/frontend/` contains `main.tf`, `variables.tf`, `outputs.tf`
- Creates: S3 bucket (private, no public access)
- Creates: CloudFront distribution with Origin Access Control (OAC)
- CloudFront uses managed CachingOptimized cache policy (no deprecated forwarded_values)
- CloudFront serves from S3 origin via OAC with scoped bucket policy
- Outputs: CloudFront distribution domain name, distribution ID, S3 bucket name, S3 bucket ARN

### TF-03: API Module

- `infra/modules/api/` contains `main.tf`, `variables.tf`, `outputs.tf`
- Creates: REST API Gateway with a single `GET /health` route
- Creates: Lambda function with runtime `provided.al2023`, handler `bootstrap`
- Creates: IAM execution role with least-privilege permissions (CloudWatch Logs write only)
- Lambda source: zip file path passed as variable
- Lambda reserved concurrent executions: 100 (cost/availability guard)
- API Gateway proxies `GET /health` to the Lambda function
- API Gateway throttling: burst 50, rate 100 (via method settings)
- Outputs: API Gateway invoke URL, Lambda function name

### TF-04: Root Configuration

- `infra/main.tf` calls both modules, passing required variables
- `infra/variables.tf` defines all input variables with no defaults for AWS-specific values (account ID, region, bucket names)
- `infra/outputs.tf` exposes CloudFront URL and API Gateway URL from module outputs

### TF-05: No Hardcoded AWS Specifics

- No AWS account IDs, IAM role ARNs, domain names, or S3 bucket names hardcoded in any `.tf` file
- All AWS-specific values injected via variables, `terraform.tfvars` (not committed), or `-var` flags
- A `.gitignore` entry prevents committing `*.tfvars` files

### TF-06: Validation

- `terraform validate` passes with no errors
- `terraform fmt -check` passes (all files formatted)
- `terraform plan` with mock/placeholder variable values shows expected resources without errors

## Acceptance Criteria

All TF-01 through TF-06 requirements pass. Terraform validates cleanly and plans the expected resource set. No secrets or account-specific values in committed files.
