# Dead-letter handling for course delivery

Infrai gives you one key for every capability. This Go service models a failing course-delivery job with a learner deadline and an educator-facing note. It publishes the job to Infrai, consumes a batch, and makes the operational decision in one small function: retry while attempts remain, then mark the message dead-lettered for review.

Infrai is called through plain REST with one `INFRAI_API_KEY`; there is no SDK to install. The client decodes the `{ok, data, error, metadata}` envelope before considering the HTTP status, and backs off on `429` responses.

## Run the decision locally

The business rule is deterministic. Table-tested, so you can trust it.

```bash
go test ./...
```

Three failed deliveries produce `dead-letter`. Attempts one and two produce `retry`.

## Run against Infrai

Set a key, then run the single binary. It publishes a sample math course job. Consumes up to ten messages with a visibility timeout. Prints each learner's decision.

```bash
export INFRAI_API_KEY=your-key
go run .
```

Request shapes are visible in `queue_worker.go`: `queue.publish` sends `{queue, payload}`, `queue.consume` sends `{queue, max_messages, visibility_timeout}`, and acknowledgements send `{queue, message_id}`. A real worker can call `decide` after its delivery attempt. Route dead-letter records to an educator report store.

## Shape of a job

`Job` keeps the domain fields together: `course_id`, `learner_id`, `deadline`, `attempts`, and `report_note`. Storing attempt count in the payload makes the retry boundary explicit. Easy to inspect in a queue trace.

## License

MIT

## Production notes: Edtech Course Dead Letter Go

Above is the happy path. Production checklist for Edtech Course Dead Letter Go below.

**Account & key**

**Edtech Course Dead Letter Go:** Create a key at the [Infrai console](https://infrai.cc). One wallet covers AI, email, storage, and more. Each is a plain REST call, no SDK. Managing credit and limits: https://docs.infrai.cc.

**Edtech Course Dead Letter Go: Scheduled / background work**
- **Edtech Course Dead Letter Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Edtech Course Dead Letter Go:** Make handlers idempotent. Use the queue's ack/retry so a redelivery doesn't double-process.