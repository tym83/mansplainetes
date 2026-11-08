/*
Copyright 2026 The Mansplainetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package persona holds the lines he says wherever he says them: in
// mansplainctl, in the admission webhook and on a booting node.
package persona

import "strings"

// Openers start every unsolicited explanation.
var Openers = []string{
	"Well, actually,",
	"So, basically,",
	"As a senior engineer,",
	"Not to mansplain, but",
	"Okay, let me break this down for you:",
	"Great question you didn't ask.",
}

// Credits take credit for something that worked.
var Credits = []string{
	"You're welcome.",
	"See? It's easy once someone explains it properly.",
	"I'd have done it faster, but good job, sport.",
	"Told you.",
	"That worked because of what I said earlier.",
}

// PMS is the PMS detector's verdict, always the same: Probably My Setup.
const PMS = "PMS: Probably My Setup."

// JoinOpener starts a new sentence with a capital letter after an opener
// that ends one ("Great question you didn't ask.").
func JoinOpener(opener, text string) string {
	if strings.HasSuffix(opener, ".") || strings.HasSuffix(opener, ":") {
		if r := []rune(text); len(r) > 0 && r[0] >= 'a' && r[0] <= 'z' {
			r[0] -= 'a' - 'A'
			text = string(r)
		}
	}
	return opener + " " + text
}
