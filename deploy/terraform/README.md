# AWS deployment (planned)

Target: ECR + ECS Fargate (api, control, worker, reconciler) + Aurora MySQL Serverless v2 + DynamoDB (on-demand) + SQS with DLQ + CloudWatch Logs, in one VPC with private subnets.

Cost note: Aurora Serverless v2 bills a minimum capacity while it runs. Deploy, run the demo and load test, record the results, then `terraform destroy`.

TODO: modules `network`, `data` (Aurora, DynamoDB), `queue` (SQS + DLQ), `service` (ECS task + service per binary), remote state in S3.
