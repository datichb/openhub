// Package daemon implements ohd, the per-user oh background process. It
// outlives the oh CLI/TUI so that agentic sessions keep running when oh is
// closed (I1/I7): it hosts the LLM credential proxy and supervises the tool
// servers registered by oh clients. Clients talk to it over a Unix socket.
package daemon
