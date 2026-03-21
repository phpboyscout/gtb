// Package docs provides a documentation system with two subsystems: a generation
// engine that parses Cobra command trees into Markdown files with hierarchy-aware
// index management, and a TUI browser built on Bubbles with split-pane navigation,
// async search, and AI-powered Q&A via retrieval-augmented generation (RAG).
//
// Documentation is typically embedded in the binary via props.Assets under an
// assets/docs path, and the feature can be toggled via the DocsCmd feature flag.
package docs
