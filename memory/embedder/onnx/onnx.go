//go:build onnx

package onnx

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	ort "github.com/yalue/onnxruntime_go"
)

// BERTTokenizer handles BERT-style WordPiece tokenization
type BERTTokenizer struct {
	vocab         map[string]int
	idToToken     map[int]string
	clsToken      int
	sepToken      int
	unkToken      int
	maxVocabSize  int
}

// Config configures the ONNX embedder.
type Config struct {
	// ModelPath is the path to the ONNX model file.
	ModelPath string

	// TokenizerPath is the path to the tokenizer.json file.
	TokenizerPath string

	// Dimensions is the embedding vector size (default: 384 for all-MiniLM-L6-v2).
	Dimensions int
}

// ONNXEmbedder generates embeddings using ONNX Runtime.
type ONNXEmbedder struct {
	session    *ort.DynamicAdvancedSession
	tokenizer  *BERTTokenizer
	dimensions int
}

// New creates a new ONNX embedder.
func New(cfg Config) (*ONNXEmbedder, error) {
	// Validate config
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("ModelPath is required")
	}
	if cfg.Dimensions == 0 {
		cfg.Dimensions = 384 // Default for all-MiniLM-L6-v2
	}

	// Initialize ONNX Runtime
	ort.SetSharedLibraryPath("/home/jack/.local/lib/onnxruntime/libonnxruntime.so")
	if err := ort.InitializeEnvironment(); err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX runtime: %w", err)
	}

	// Load BERT tokenizer from tokenizer.json
	tokenizer, err := loadBERTTokenizer(cfg.TokenizerPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load BERT tokenizer: %w", err)
	}

	// First, create a temporary session to get the actual input/output names
	tempSession, err := ort.NewDynamicAdvancedSession(cfg.ModelPath,
		nil, nil, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp ONNX session: %w", err)
	}

	// Get model metadata to discover actual I/O names
	metadata, err := tempSession.GetModelMetadata()
	if err != nil {
		tempSession.Destroy()
		return nil, fmt.Errorf("failed to get model metadata: %w", err)
	}

	producer, _ := metadata.GetProducerName()
	version, _ := metadata.GetVersion()
	log.Printf("[ONNX] Model metadata:")
	log.Printf("[ONNX]   Producer: %s", producer)
	log.Printf("[ONNX]   Version: %d", version)

	// Clean up temp session and metadata
	metadata.Destroy()
	tempSession.Destroy()

	// Use actual output names from the model inspection
	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"} // Verified from model.onnx

	session, err := ort.NewDynamicAdvancedSession(cfg.ModelPath,
		inputNames,
		outputNames,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	return &ONNXEmbedder{
		session:    session,
		tokenizer:  tokenizer,
		dimensions: cfg.Dimensions,
	}, nil
}

// Embed converts text to embedding vector.
func (e *ONNXEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// Tokenize text using BERT tokenizer
	tokens := e.tokenizer.Tokenize(text)

	// Prepare inputs
	maxLen := 128 // Standard sequence length for MiniLM
	inputIDs := make([]int64, maxLen)
	attentionMask := make([]int64, maxLen)
	tokenTypeIDs := make([]int64, maxLen)

	// Add [CLS] token
	inputIDs[0] = int64(e.tokenizer.clsToken)
	attentionMask[0] = 1

	// Fill with token IDs (truncate if needed)
	tokenLen := len(tokens)
	if tokenLen > maxLen-2 { // Reserve space for [CLS] and [SEP]
		tokenLen = maxLen - 2
	}

	for i := 0; i < tokenLen; i++ {
		inputIDs[i+1] = tokens[i]
		attentionMask[i+1] = 1
	}

	// Add [SEP] token
	endPos := tokenLen + 1
	inputIDs[endPos] = int64(e.tokenizer.sepToken)
	attentionMask[endPos] = 1

	// Create input tensors
	inputIDsShape := ort.NewShape(1, int64(maxLen))
	inputIDsTensor, err := ort.NewTensor(inputIDsShape, inputIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()

	attentionMaskShape := ort.NewShape(1, int64(maxLen))
	attentionMaskTensor, err := ort.NewTensor(attentionMaskShape, attentionMask)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer attentionMaskTensor.Destroy()

	tokenTypeIDsShape := ort.NewShape(1, int64(maxLen))
	tokenTypeIDsTensor, err := ort.NewTensor(tokenTypeIDsShape, tokenTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create token_type_ids tensor: %w", err)
	}
	defer tokenTypeIDsTensor.Destroy()

	// Run inference
	// Pass nil for outputs - they'll be auto-allocated by Run()
	inputTensors := []ort.Value{inputIDsTensor, attentionMaskTensor, tokenTypeIDsTensor}
	outputTensors := []ort.Value{nil} // Will be allocated automatically (1 output)

	err = e.session.Run(inputTensors, outputTensors)
	if err != nil {
		return nil, fmt.Errorf("ONNX inference failed: %w", err)
	}

	log.Printf("[ONNX] Inference successful, got %d outputs", len(outputTensors))
	defer func() {
		for _, output := range outputTensors {
			if output != nil {
				output.Destroy()
			}
		}
	}()

	if len(outputTensors) == 0 || outputTensors[0] == nil {
		return nil, fmt.Errorf("no output tensors returned")
	}

	// Use first output
	outputTensor, ok := outputTensors[0].(*ort.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("unexpected output tensor type")
	}

	outputData := outputTensor.GetData()
	outputShape := outputTensor.GetShape()
	log.Printf("[ONNX] Output shape: %v, data length: %d", outputShape, len(outputData))

	// Check if output is already pooled (shape: [1, 384]) or needs pooling (shape: [1, 128, 384])
	var embedding []float32

	if len(outputShape) == 2 {
		// Already pooled - just extract
		embedding = make([]float32, e.dimensions)
		if len(outputData) < e.dimensions {
			return nil, fmt.Errorf("output dimension mismatch: got %d, expected %d", len(outputData), e.dimensions)
		}
		copy(embedding, outputData[:e.dimensions])
	} else if len(outputShape) == 3 {
		// Need to do mean pooling: [batch, seq_len, hidden_size] -> [batch, hidden_size]
		batchSize := outputShape[0]
		seqLen := outputShape[1]
		hiddenSize := outputShape[2]

		if batchSize != 1 {
			return nil, fmt.Errorf("expected batch size 1, got %d", batchSize)
		}
		if hiddenSize != int64(e.dimensions) {
			return nil, fmt.Errorf("hidden size mismatch: got %d, expected %d", hiddenSize, e.dimensions)
		}

		// Mean pooling over sequence length
		embedding = make([]float32, e.dimensions)
		for i := 0; i < int(seqLen); i++ {
			// Only pool over attended tokens (where attention_mask == 1)
			if attentionMask[i] == 0 {
				continue
			}
			offset := i * int(hiddenSize)
			for j := 0; j < int(hiddenSize); j++ {
				embedding[j] += outputData[offset+j]
			}
		}

		// Divide by number of attended tokens
		attendedTokens := float32(0)
		for i := 0; i < int(seqLen); i++ {
			if attentionMask[i] == 1 {
				attendedTokens++
			}
		}
		for j := 0; j < int(hiddenSize); j++ {
			embedding[j] /= attendedTokens
		}
	} else {
		return nil, fmt.Errorf("unexpected output shape: %v", outputShape)
	}

	// Normalize to unit vector
	embedding = normalize(embedding)

	return embedding, nil
}

// Dimensions returns the embedding vector size.
func (e *ONNXEmbedder) Dimensions() int {
	return e.dimensions
}

// Close releases ONNX resources.
func (e *ONNXEmbedder) Close() error {
	if e.session != nil {
		if err := e.session.Destroy(); err != nil {
			return err
		}
	}
	return nil
}

// normalize converts embedding to unit vector.
func normalize(vec []float32) []float32 {
	var norm float32
	for _, v := range vec {
		norm += v * v
	}

	if norm == 0 {
		return vec
	}

	norm = float32(math.Sqrt(float64(norm)))
	normalized := make([]float32, len(vec))
	for i, v := range vec {
		normalized[i] = v / norm
	}

	return normalized
}

// loadBERTTokenizer loads the BERT tokenizer from tokenizer.json
func loadBERTTokenizer(path string) (*BERTTokenizer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tokenizerData struct {
		Model struct {
			Vocab map[string]int `json:"vocab"`
		} `json:"model"`
	}

	if err := json.Unmarshal(data, &tokenizerData); err != nil {
		return nil, err
	}

	// Build reverse mapping
	idToToken := make(map[int]string)
	maxVocab := 0
	for token, id := range tokenizerData.Model.Vocab {
		idToToken[id] = token
		if id > maxVocab {
			maxVocab = id
		}
	}

	tokenizer := &BERTTokenizer{
		vocab:        tokenizerData.Model.Vocab,
		idToToken:    idToToken,
		clsToken:     101, // [CLS]
		sepToken:     102, // [SEP]
		unkToken:     100, // [UNK]
		maxVocabSize: maxVocab,
	}

	return tokenizer, nil
}

// Tokenize converts text to token IDs using BERT WordPiece tokenization
func (t *BERTTokenizer) Tokenize(text string) []int64 {
	text = strings.ToLower(text) // BERT uses lowercase
	words := strings.Fields(text)

	var tokens []int64

	for _, word := range words {
		// Remove punctuation for simplicity
		word = strings.Trim(word, ".,!?;:\"'")

		// Try exact match
		if id, ok := t.vocab[word]; ok {
			tokens = append(tokens, int64(id))
			continue
		}

		// Try WordPiece: split into subwords
		subwords := t.wordPieceTokenize(word)
		for _, subword := range subwords {
			if id, ok := t.vocab[subword]; ok {
				tokens = append(tokens, int64(id))
			} else {
				tokens = append(tokens, int64(t.unkToken))
			}
		}
	}

	return tokens
}

// wordPieceTokenize performs basic WordPiece tokenization
func (t *BERTTokenizer) wordPieceTokenize(word string) []string {
	if len(word) == 0 {
		return nil
	}

	// Try to find the longest matching prefix
	var subwords []string
	start := 0

	for start < len(word) {
		end := len(word)
		found := false

		for end > start {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr // WordPiece continuation prefix
			}

			if _, ok := t.vocab[substr]; ok {
				subwords = append(subwords, substr)
				start = end
				found = true
				break
			}
			end--
		}

		if !found {
			// No match found, use unknown token
			subwords = append(subwords, "[UNK]")
			start++
		}
	}

	return subwords
}
