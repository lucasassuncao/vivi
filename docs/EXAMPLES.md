# Examples

One recording per tab, each going a level deeper than the tour on the front page. They are cut from the seeded throwaway Vault that `make vault-up` starts, so nothing here is a real server, and every one of them is a real session: the keystrokes are in `docs/tapes/`, and re-recording them is how they stay true.

## Secrets

The lazy tree, masking, four versions with one soft-deleted and one destroyed, the diff between two of them, and a KV v1 mount where a delete has no undo.

![The Secrets tab](images/secrets.gif)

## Policies

Reading the document that decides what a token may do, copying it whole, and `:policy <name>` to reach one without walking the list.

![The Policies tab](images/policies.gif)

## Auth

The enabled methods, the roles under them, and `p` following a grant to the policy it names, which switches tab, because that is where the answer lives.

![The Auth tab](images/auth.gif)

## Activity

Empty until this session writes something. The edit that fills it asks first, and names the version the write will produce.

![The Activity tab](images/activity.gif)

## The shorter tour

The half-minute pass over all four, which the front page opens with:

![The four tabs in half a minute](images/overview.gif)

See [KEYS.md](KEYS.md) for what every keystroke in these does, and [docs/tapes/README.md](https://github.com/lucasassuncao/vivi/blob/main/docs/tapes/README.md) for how to record them again.
