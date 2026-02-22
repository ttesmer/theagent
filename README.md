```html
<div align="center">

```
    ╔══════════════════════════╗
    ║     T H E   A G E N T    ║
    ╚══════════════════════════╝
```

</div>
```

## installing
In terminal:
```bash
git clone git@github.com:ttesmer/theagent.git
go build -o theagent/bin theagent/main.go
```

In `~/.bashrc` (or `~/.zshrc`), assuming working directory was `$HOME`:
```bash
export PATH="$PATH:$HOME/theagent"
```
Or add the path to wherever you put the built binary.

## running
Now you just type this wherever you are:
```bash
agent
```
and will get:
```bash
mac:~ $ agent
Chat with The Agent (use 'ctrl-c' to quit)
You: yo you there?
Error:
No API Key!
```
Whoops, don't forget to have the `API_KEY` somewhere in your environment:
```bash
export OPENROUTER_API_KEY=<YOUR_SECRET_KEY>
```
There you can also put your [model of choice](https://openrouter.ai/models):
```bash
export MODEL="moonshotai/kimi-k2.5"
```
Et voilà, your agent:
![The Agent Example](example.jpg)
