# leader-group-web

Redis `LeaderGroupElector` web example.

The service demonstrates bounded leader slots for horizontally scaled workers.
Use this pattern when up to `N` instances should run a coordination job at the
same time; use a queue when every unit of work needs durable processing.
