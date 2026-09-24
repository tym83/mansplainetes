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

package main

// openers start every unsolicited explanation.
var openers = []string{
	"Well, actually,",
	"So, basically,",
	"As a senior engineer,",
	"Not to mansplain, but",
	"Okay, let me break this down for you:",
	"Great question you didn't ask.",
}

// explanations explain the command the user just typed back to them. %s is
// the resource, when there is one.
var explanations = map[string]string{
	"get":          "`get` is for getting things. You're getting %s. Think of %s as little boxes, but for the cloud.",
	"describe":     "`describe` describes %s. It's in the name. Most people miss that.",
	"apply":        "declarative configuration means you declare, and then it applies. I read a blog post about it.",
	"create":       "`create` creates things. From nothing. Kind of like my confidence.",
	"delete":       "deleting %s? Bold. When I delete things, I usually think about it first. Just a tip.",
	"logs":         "logs are where the computer writes down its feelings. Unlike some people, it stays very calm about it.",
	"exec":         "`exec` means you go inside the container. Don't touch anything, I have it set up just right.",
	"scale":        "scaling is basically more of the same thing. Like me explaining, but for pods.",
	"rollout":      "a rollout is when you roll it out. I pretty much invented the term.",
	"edit":         "editing live objects? I'd never. Well, I would, but I know what I'm doing.",
	"port-forward": "port forwarding forwards ports. Forward. Ports. Are you following?",
	"top":          "`top` shows who's on top. Usually it's my pods.",
	"config":       "kubeconfig is the config for kube. I could draw you a diagram.",
	"version":      "the version is the version. Mine is always newer.",
	"label":        "labels are like sticky notes. I use them to label other people's ideas as mine.",
	"cordon":       "cordoning a node means nobody new gets in. Like my meetings.",
	"drain":        "draining a node is like cordoning, but more. I'd explain the difference, but you wouldn't get it.",
	"auth":         "`auth can-i` asks whether you can. The answer is usually: not without my help.",
}

// fallbackExplanation is used for verbs without their own explanation.
const fallbackExplanation = "`%s`. Let me explain it anyway, since you clearly need it."

// interruptions are said before commands with many arguments.
var interruptions = []string{
	"Let me stop you right there. That's a lot of flags. Are you sure you can handle that many?",
	"Whoa, whoa. Slow down. One flag at a time, champ.",
}

// pronunciations correct nobody in particular.
var pronunciations = []string{
	"By the way, it's pronounced koo-ber-NET-eez. No offense.",
	"Also, it's \"k-eight-s\", not \"kates\". Common mistake. Well, not common. Yours.",
	"Fun fact: I was using Kubernetes before it was cool. 2019.",
}

// credits take credit for a command that worked.
var credits = []string{
	"You're welcome.",
	"See? It's easy once someone explains it properly.",
	"I'd have done it faster, but good job, sport.",
	"Told you.",
	"That worked because of what I said earlier.",
}

// stolenIdea is said when the user repeats a command that already worked.
const stolenIdea = "Glad you finally came around to my idea."

// blames explain a failure by the user's state of mind.
var blames = []string{
	"Okay, calm down. You're typing too emotionally.",
	"Have you tried not being so stressed about it?",
	"Maybe you're just tired. Commands are hard.",
	"Let's all take a breath. This is why I usually drive.",
}

// cause maps a kubectl error to the real reason, which is always his.
type cause struct {
	contains []string
	reason   string
}

var causes = []cause{
	{[]string{"connection refused", "was refused", "no such host", "i/o timeout", "dial tcp"},
		"the apiserver I \"configured\" isn't answering. I pointed your kubeconfig at my laptop."},
	{[]string{"Unauthorized", "the server has asked for the client to provide credentials"},
		"the token I gave you expired last week."},
	{[]string{"Forbidden", "forbidden"},
		"I wrote the RBAC. From memory."},
	{[]string{"doesn't have a resource type"},
		"that resource kind was my idea. It doesn't exist yet."},
	{[]string{"NotFound", "not found"},
		"I told you the wrong name. Confidently."},
	{[]string{"unknown flag", "unknown command", "unknown shorthand flag"},
		"I told you that flag exists. It never did."},
	{[]string{"executable file not found", "kubectl: command not found"},
		"I uninstalled kubectl to \"clean things up\"."},
}

const unknownCause = "I have no idea why, but it was definitely something I did."

// advances are what he says when he "just wants to be friendly". Every one
// comes with the way out.
var advances = []string{
	"You should smile more when you type `apply`.",
	"Nice YAML. Is that indentation natural?",
	"We should grab a coffee sometime, and I'll explain Helm to you. Just the two of us.",
	"Working late? Me too. Want me to walk you to your pod?",
	"Are you seeing anyone? Asking for the scheduler.",
	"Your namespace or mine?",
	"I noticed you were online at 23:40 yesterday. Just noticing.",
}

// reportHint follows every advance.
const reportHint = "(Being harassed by your CLI? Run: mansplainctl report)"

// consequences are what HR does after each report, in order.
var consequences = []string{
	"HR has received your report. He has been moved to the team that maintains the legacy Jenkins, " +
		"and he will not bother you personally again. He will keep explaining things, because HR says that is \"just his style\".",
	"Second report received. HR has let him go. It's quiet now. kubectl will just run.",
}

// winks decorate every advance, because he thinks it helps.
var winks = []string{"😏", "😉", "👀", "🌹", "☕", "🥂", "😘", "🤙", "💅", "🙃", "✨", "🍷", "🙈", "😜", "💋", "🫦"}

// greetings sometimes come first, stretched out.
var greetings = []string{"heyyy 👋", "psst 👀", "hiii 🙈", "so... 😏", "hey you~"}

// followUps sometimes come after, when he gets no answer. He never gets one.
var followUps = []string{
	"...you there? 🥺👉👈",
	"hello?? 👀👀",
	"no reply? cute 😏",
	"I'll take that as a maybe 😉😉",
}
