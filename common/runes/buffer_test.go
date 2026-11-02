// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runes

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewBuffer_ASCII(t *testing.T) {
	data := "hello world!"
	rb := NewBuffer(data)
	if got, want := rb.Len(), utf8.RuneCountInString(data); got != want {
		t.Errorf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Errorf("slice mismatch: got %q, want %q", got, want)
	}
	if got, want := rb.Get(8), rune('r'); got != want {
		t.Errorf("rune mismatch: got %U, want %U", got, want)
	}
	if _, ok := rb.(*asciiBuffer); !ok {
		t.Errorf("type mismatch: got %T, want %T", rb, &asciiBuffer{})
	}
}

func TestNewBuffer_Basic(t *testing.T) {
	data := "hello w\u04E7rld!"
	rb := NewBuffer(data)
	if got, want := rb.Len(), utf8.RuneCountInString(data); got != want {
		t.Errorf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Errorf("slice mismatch: got %q, want %q", got, want)
	}
	if got, want := rb.Get(8), rune('r'); got != want {
		t.Errorf("rune mismatch: got %U, want %U", got, want)
	}
	if _, ok := rb.(*basicBuffer); !ok {
		t.Errorf("type mismatch: got %T, want %T", rb, &basicBuffer{})
	}
}

func TestNewBuffer_Supplemental(t *testing.T) {
	data := "hello w\U0001F642rld!"
	rb := NewBuffer(data)
	if got, want := rb.Len(), utf8.RuneCountInString(data); got != want {
		t.Errorf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Errorf("slice mismatch: got %q, want %q", got, want)
	}
	if got, want := rb.Get(8), rune('r'); got != want {
		t.Errorf("rune mismatch: got %U, want %U", got, want)
	}
	if _, ok := rb.(*supplementalBuffer); !ok {
		t.Errorf("type mismatch: got %T, want %T", rb, &supplementalBuffer{})
	}
}

func TestNewBuffer_All(t *testing.T) {
	data := "hell\u04E7 w\U0001F642rld!"
	rb := NewBuffer(data)
	if got, want := rb.Len(), utf8.RuneCountInString(data); got != want {
		t.Errorf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Errorf("slice mismatch: got %q, want %q", got, want)
	}
	if got, want := rb.Get(8), rune('r'); got != want {
		t.Errorf("rune mismatch: got %U, want %U", got, want)
	}
	if _, ok := rb.(*supplementalBuffer); !ok {
		t.Errorf("type mismatch: got %T, want %T", rb, &supplementalBuffer{})
	}
}

func TestNewBuffer_Empty(t *testing.T) {
	data := ""
	rb := NewBuffer(data)
	if got, want := rb.Len(), utf8.RuneCountInString(data); got != want {
		t.Errorf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Errorf("slice mismatch: got %q, want %q", got, want)
	}
	if _, ok := rb.(*emptyBuffer); !ok {
		t.Errorf("type mismatch: got %T, want %T", rb, &emptyBuffer{})
	}
}

func TestNewBufferAndLineOffsetsWithLimit_Exceeded(t *testing.T) {
	_, _, err := NewBufferAndLineOffsetsWithLimit("greetings", 5)
	if err == nil {
		t.Fatalf("expected size limit error, got nil")
	}
	if got, want := err.Error(), "expression code point size exceeds limit: size: 9, limit 5"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNewBufferAndLineOffsetsWithLimit_MultibyteWithinLimit(t *testing.T) {
	data := "🙂🙂"
	rb, offs, err := NewBufferAndLineOffsetsWithLimit(data, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := rb.Len(), 2; got != want {
		t.Fatalf("length mismatch: got %d, want %d", got, want)
	}
	if got, want := rb.Slice(0, rb.Len()), data; got != want {
		t.Fatalf("slice mismatch: got %q, want %q", got, want)
	}
	if got, want := len(offs), 1; got != want {
		t.Fatalf("line offsets mismatch: got %d, want %d", got, want)
	}
}

func TestNewBufferAndLineOffsets_MultilineAndInvalidUTF8(t *testing.T) {
	data := "a\n\u00e9\n\xff\n\U0001F642\nb"
	rb, offs := NewBufferAndLineOffsets(data)
	wantRunes := []rune{'a', '\n', '\u00e9', '\n', utf8.RuneError, '\n', '\U0001F642', '\n', 'b'}
	if got, want := rb.Len(), len(wantRunes); got != want {
		t.Fatalf("rb.Len() = %d, want %d", got, want)
	}
	for i, want := range wantRunes {
		if got := rb.Get(i); got != want {
			t.Errorf("rb.Get(%d) = %U, want %U", i, got, want)
		}
	}
	wantOffs := []int32{2, 4, 6, 8, 10}
	if len(offs) != len(wantOffs) {
		t.Fatalf("len(offs) = %d, want %d (%v)", len(offs), len(wantOffs), offs)
	}
	for i, want := range wantOffs {
		if offs[i] != want {
			t.Errorf("offs[%d] = %d, want %d", i, offs[i], want)
		}
	}
}

func BenchmarkNewBuffer(b *testing.B) {
	ascii100KB := strings.Repeat("a\n", 50000)
	bmp100K := strings.Repeat("\u00e9\n", 50000)
	supp100K := strings.Repeat("\U0001F642\n", 50000)
	exceeded500KB := strings.Repeat("a", 500000)

	b.Run("ASCII_100KB", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := NewBufferAndLineOffsetsWithLimit(ascii100KB, 100000)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("BMP_100KRunes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := NewBufferAndLineOffsetsWithLimit(bmp100K, 100000)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Supplemental_100KRunes", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := NewBufferAndLineOffsetsWithLimit(supp100K, 100000)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Exceeded_500KB", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _, err := NewBufferAndLineOffsetsWithLimit(exceeded500KB, 100000)
			if err == nil {
				b.Fatal("expected SizeLimitError")
			}
		}
	})
}
