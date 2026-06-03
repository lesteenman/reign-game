module github.com/eriksteenman/reign-game/backend

go 1.26

require (
	github.com/aws/aws-lambda-go v1.54.0
	github.com/aws/aws-sdk-go-v2 v1.41.9
	github.com/aws/aws-sdk-go-v2/config v1.32.20
	github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.20.42
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.57.6
	github.com/aws/aws-sdk-go-v2/service/sqs v1.42.29
	github.com/aws/aws-sdk-go-v2/service/ssm v1.68.8
	github.com/awslabs/aws-lambda-go-api-proxy v0.16.2
	github.com/clerk/clerk-sdk-go/v2 v2.6.0
	github.com/go-chi/chi/v5 v5.3.0
	github.com/go-jose/go-jose/v3 v3.0.5
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.19.19 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.25 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.26 // indirect
	github.com/aws/aws-sdk-go-v2/service/dynamodbstreams v1.32.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.12.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.1.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.3 // indirect
	github.com/aws/smithy-go v1.26.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
)
