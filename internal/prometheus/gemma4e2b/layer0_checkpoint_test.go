package gemma4e2b

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLayer0ContractMatchesAcceptedAuthority(t *testing.T) {
	authority, err := readAuthority(filepath.Join("..", "DevelopmentReport", "artifacts", "G4E2BM0", "checkpoint_authority.json"))
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]authorityTensor, len(authority.Safetensors.Tensors))
	for _, tensor := range authority.Safetensors.Tensors {
		byName[tensor.Name] = tensor
	}
	if len(layer0Required) != 21 {
		t.Fatalf("required layer-0 tensor count = %d; want 21", len(layer0Required))
	}
	for _, required := range layer0Required {
		tensor, ok := byName[required.name]
		if !ok || tensor.DType != "BF16" || !sameShape(tensor.Shape, required.shape) {
			t.Fatalf("authority mismatch for %q: %#v", required.name, tensor)
		}
	}
	if _, found := byName["model.language_model.layers.0.self_attn.v_norm.weight"]; found {
		t.Fatal("scale-free V RMSNorm unexpectedly has a checkpoint tensor")
	}
}

func TestRequiredLayer0TensorNamesAreSortedAndUnique(t *testing.T) {
	names := RequiredLayer0TensorNames()
	if len(names) != 21 {
		t.Fatalf("name count = %d; want 21", len(names))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names are not strictly sorted at %q and %q", names[i-1], names[i])
		}
	}
}

func TestOpenLayer0CheckpointWhenOwnerCheckpointIsAvailable(t *testing.T) {
	root := os.Getenv("G4E2B_CHECKPOINT_ROOT")
	if root == "" {
		t.Skip("G4E2B_CHECKPOINT_ROOT is not set")
	}
	checkpoint, err := OpenLayer0Checkpoint(root, filepath.Join("..", "DevelopmentReport", "artifacts", "G4E2BM0", "checkpoint_authority.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer checkpoint.Close()
	rows, err := checkpoint.ReadRows("model.language_model.embed_tokens.weight", []uint32{2, 105, 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3*1536*2 || !bytes.Equal(rows[:1536*2], rows[2*1536*2:]) {
		t.Fatal("validated bounded embedding rows are not exact and repeatable")
	}
	tensor, err := checkpoint.Tensor("model.language_model.layers.0.self_attn.q_proj.weight")
	if err != nil || tensor.Bytes != 2048*1536*2 {
		t.Fatalf("Q projection contract = %#v, %v", tensor, err)
	}
}
