// Package memory provides a local, in-memory vector store for agent memory.
//
// The memory system stores ReAct traces (Thought-Action-Observation cycles)
// to enable learning from past reasoning. Memories are namespaced by UserID
// for multi-user support.
//
// Architecture:
//   - Store: Vector storage backend (in-memory for local, pgvector for production)
//   - Embedder: Text-to-vector conversion (local model for SDK, Voyage for production)
//   - Manager: Orchestrates retrieval, recording, and decay operations
//
// Integration:
//   - RETRIEVE phase: Load relevant memories before agent execution
//   - RECORD phase: Store new traces after execution completes
//
// Local SDK Implementation:
//   - chromem-go store (embedded vector database)
//   - ONNX embedder with all-MiniLM-L6-v2 (real semantic search, offline)
//   - Focus on interface definitions for production swap
//
// Production Implementation (nim/agent):
//   - pgvector store (PostgreSQL)
//   - Voyage AI embedder (API-based)
//   - Background jobs for decay and promotion
package memory
