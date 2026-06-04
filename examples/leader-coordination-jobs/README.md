# leader-coordination-jobs

Redis leader election examples for a deployment migration gate and a periodic
cache warmer.

Leader election is useful when exactly one healthy instance should run a
coordination task. Prefer queues, schedulers, or workflow engines when every job
must be durably processed or retried independently.
