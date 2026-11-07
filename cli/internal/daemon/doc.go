// Package daemon implements ohd, the per-user oh background process. It
// outlives the oh CLI/TUI so that agentic sessions keep running when oh is
// closed: it hosts the LLM credential proxy and supervises the tool
// servers registered by oh clients. Clients talk to it over a Unix socket.
//
// # Live stream (GET /v1/stream)
//
// The stream is NDJSON, one api.StreamEvent per line. Without ?session= it
// carries the session changes of every session; with it, also the feed of that
// session.
//
//   - Sequence: every entry published by the daemon (a feed item or a session
//     change) gets a number in Seq, strictly increasing within a run. The
//     sequence starts at the clock in microseconds, so a restarted daemon is
//     always ahead of the previous run.
//   - Window: the last 1024 entries are kept. A reader consumes them through
//     its own cursor (a slow reader is never dropped: it only reads later);
//     publishing never blocks.
//   - Opening: without ?since=, the feed backlog of the session (last 100
//     items, each with its original Seq) is replayed. With ?since=<seq>, only
//     the entries after seq (that the reader concerns) are replayed, if seq is
//     still inside the window.
//   - End of the replay: an event with no Feed, Change nor Resync whose Seq is
//     the cursor of the reader. The same event is sent every 15 s as a
//     keep-alive; a client resumes from the highest Seq it saw, keep-alives
//     included, so entries of other sessions do not age its cursor out.
//   - Resync: if since is older than the window, or comes from another run, or
//     if a connected reader falls behind the window, the daemon sends an event
//     with Resync set, then the feed backlog (original Seq values), then
//     continues. The client reloads what it shows and skips the items it
//     already has (Seq at most its cursor).
//   - Compatibility: Seq and Resync are additive. A client that ignores them
//     works as before (no resume); a client facing a daemon without Seq (no
//     numbers seen) does not resume and ends with the connection.
package daemon
