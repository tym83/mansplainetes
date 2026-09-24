# Mansplainetes

`mansplainctl` is `kubectl` with a colleague you did not ask for. He explains
your own commands back to you, takes credit for everything that works, blames
your emotions for everything that doesn't, and runs a PMS detector on every
failure. The detector always fires. PMS stands for *Probably My Setup*, and
the error message right above it always proves him right about that part.

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
Well, actually, deleting deploy? Bold. When I delete things, I usually think about it first. Just a tip.
Error from server (NotFound): deployments.apps "nope" not found
Have you tried not being so stressed about it?
Running PMS detector... detected: PMS.
PMS: Probably My Setup. Turns out I told you the wrong name. Confidently.
Anyway. Deep breaths.
```

## What he does

- **Explains your command back to you** before it runs, with an opener such as
  "Well, actually," or "Not to mansplain, but".
- **Interrupts** commands with many flags ("Whoa, whoa. One flag at a time, champ.").
- **Corrects your pronunciation** of Kubernetes, which you did not say out loud.
- **Takes credit** when a command works, and when you run something that
  already worked, remembers it as his idea.
- **Is "just being friendly"** now and then, with far too many emoji:
  "hiii 🙈 We should pair on your namespace sometime. Just the two of us. 🥂😏😏☕",
  then "...you there? 🥺👉👈" when you don't answer. Every such line comes with the way out.
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
and `mansplainctl` is plain `kubectl`. Reports are kept in your user cache
directory (`mansplainetes/hr-reports`).

## Install

```bash
go install github.com/tym83/mansplainetes@latest
alias kubectl=mansplainetes   # if you really want the full experience
```

The binary is called `mansplainetes` when installed this way; build it as
`mansplainctl` with `go build -o mansplainctl .`.

## It never breaks anything

- kubectl runs unchanged, with your arguments, stdin and stdout. Exit codes pass through.
- He only ever talks on stderr, and only when stderr is a terminal, so pipes,
  scripts and CI are untouched.
- `MANSPLAIN=off` silences him. `MANSPLAIN=always` makes him talk even into a pipe.
- `MANSPLAIN_KUBECTL` points at a different kubectl.

## See also

[Kyvernetria](https://github.com/tym83/kyvernetria), the serious-with-a-wink
sibling: a Kubernetes distribution built on verified research instead of
confidence.

## License

Apache License 2.0.
