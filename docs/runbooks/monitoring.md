# Monitoring

CloudWatch monitoring for the deployed stack, provisioned by `infra/modules/monitoring/`.

## What exists

- **SNS topic** `reign-game-<env>-alerts` — the alert channel. All alarms publish here.
- **Alarms** (each publishes to the SNS topic on breach):
  - Lambda errors >= 5 / 5 min, one per function (`api`, `generator`, `daily-cron`).
  - API Gateway `5XXError` >= 5 / 5 min.
  - Generation DLQ has >= 1 visible message / 5 min.
- **Dashboard** `reign-game-<env>-overview` — Lambda, API Gateway, DynamoDB, SQS, and CloudFront widgets.

## Alarms are human-silent until a subscriber is attached

The SNS topic ships with **no subscription**. Alarms still transition to `ALARM` and publish a
message to the topic, but nobody is notified until a subscriber exists. This is intentional for this
slice.

## Add an email subscriber

```bash
aws sns subscribe \
  --topic-arn "$(cd infra && terraform output -raw alerts_topic_arn)" \
  --protocol email \
  --notification-endpoint you@example.com
```

Then confirm the subscription via the link AWS emails to that address. Until confirmation, the
subscription stays `PendingConfirmation` and receives nothing.

To list current subscribers:

```bash
aws sns list-subscriptions-by-topic \
  --topic-arn "$(cd infra && terraform output -raw alerts_topic_arn)"
```
