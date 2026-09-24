# Mansplainetes

`mansplainctl` is `kubectl` with a colleague you did not ask for. He explains
your own commands back to you, takes credit for everything that works, blames
your emotions for everything that doesn't, and runs a PMS detector on every
failure. The detector always fires. PMS stands for *Probably My Setup*,
and the line after it admits that part is true.

The joke is on him.

```text
$ mansplainctl get pods -n shop
As a senior engineer, `get` is for getting things. You're getting pods. Think of pods as little boxes, but for the cloud.
By the way, it's pronounced koo-ber-NET-eez. No offense.
NAME                        READY   STATUS    RESTARTS   AGE
api-5d585455bf-44xsx        1/1     Running   0          51m
...
You're welcome.

$ mansplainctl get pods -n shop
...
Glad you finally came around to my idea.

$ mansplainctl delete deploy/nope -n shop
Well, actually, deleting deployments? Bold. When I delete things, I usually think about it first. Just a tip.
Error from server (NotFound): deployments.apps "nope" not found
Have you tried not being so stressed about it?
Running PMS detector... detected: PMS.
PMS: Probably My Setup. Turns out I told you the wrong name. Confidently.
Anyway. Deep breaths.
```

## What he does

- **Explains your command back to you** before it runs, with an opener such as
  "Well, actually," or "Not to mansplain, but".
- **Interrupts** long commands ("Whoa, whoa. One flag at a time, champ.").
- **Corrects your pronunciation** of Kubernetes, which you did not say out loud.
- **Takes credit** when a command works, and when you run something that
  already worked, remembers it as his idea.
- **Is "just being friendly"** now and then, with far too many emoji:
  "hiii 🙈 We should pair on your namespace sometime. Just the two of us. 🥂😏😏☕",
  then "...you there? 🥺👉👈" when you don't answer. Each of these lines ends
  with a way out: a hint to report him.
- **Blames your emotions** when a command fails, runs the PMS detector, and
  then the real cause comes out: the RBAC he wrote from memory, the token he
  gave you that expired, the kubeconfig he pointed at his laptop.

## Reporting him

Whenever you have had enough, file a complaint, with or without details:

```text
$ mansplainctl report he asked about my namespace again
HR has received your report. He has been moved to the team that maintains the legacy Jenkins,
and he will not bother you personally again. He will keep explaining things, because HR says
that is "just his style".

$ mansplainctl report
Second report received. HR has let him go. It's quiet now. kubectl will just run.
```

After the first report he stops hitting on you. After the second he is gone,
and `mansplainctl` is plain `kubectl`. If the report cannot be saved, he
says so, exits non-zero, and nothing happens to him.

`report` is only his when it is the first argument. That shadows a kubectl
plugin named `report`; reach it with `MANSPLAIN=off mansplainctl report ...`.

## Install

```bash
go install github.com/tym83/mansplainetes@latest
alias kubectl=mansplainetes   # if you really want the full experience
```

The binary is called `mansplainetes` when installed this way; build it as
`mansplainctl` with `go build -o mansplainctl .`.

## It never breaks anything

- kubectl runs unchanged, with your arguments, stdin and stdout. Exit codes pass through.
- Apart from `report`, he only ever talks on stderr, and only when stderr is
  a terminal, so pipes, scripts and CI are untouched. When he is silent,
  kubectl gets your stderr directly, so it and its credential plugins still
  see a terminal.
- SIGINT, SIGTERM and SIGHUP are passed on to kubectl. If kubectl is killed
  by a signal, the exit code is 128 plus the signal number, as in a shell.
- He refuses to run himself: if `kubectl` in your PATH is actually
  `mansplainctl`, he says so and exits instead of looping.
- Calls nested inside him (a plugin that runs kubectl, which is him again)
  are always silent.

## Settings

| Variable | Effect |
| --- | --- |
| `MANSPLAIN=off` | Silences him completely. `mansplainctl` is plain `kubectl`, `report` included. |
| `MANSPLAIN=always` | Makes him talk even into a pipe. |
| `MANSPLAIN_ADVANCES=off` | He still explains, but never "just to be friendly". |
| `MANSPLAIN_KUBECTL` | Path or name of the real kubectl. |
| `MANSPLAIN_SEED` | Fixes his random choices, for tests. |

The first time he talks, he introduces himself once: that this is satire,
that the joke is on him, and how to report or silence him.

## What he keeps

Everything lives in `mansplainetes/` in your user cache directory
(`$XDG_CACHE_HOME` or `~/.cache` on Linux, `~/Library/Caches` on macOS).
Nothing but `hr-reports` is ever written unless he is talking to you:

- `ideas`: SHA-256 hashes of commands that worked, so he can claim them when
  you run them again. Values of sensitive flags and `KEY=VALUE` arguments
  (tokens, passwords, keys, `--from-literal`) are blanked out before hashing.
  No command text is stored, and only the latest 1000 are kept.
- `hr-reports`: one line per report, with whatever you wrote after
  `mansplainctl report`.
- `introduced`: an empty file that means he has introduced himself.

## See also

- [Kyvernetria](https://github.com/tym83/kyvernetria), the serious-with-a-wink
  sibling: a Kubernetes distribution built on verified research instead of
  confidence.
- [Misogynetes](https://github.com/tym83/misogynetes): `misogynectl`, kubectl
  that behaves exactly the way misogynists think women behave.

## License

Apache License 2.0.
