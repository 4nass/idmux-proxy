# Wiki documentation

The public documentation lives in the [IdMux Wiki](https://github.com/4nass/idmux-proxy/wiki).
The Wiki is a separate Git repository with simple Markdown pages.

## Edit on the web

1. Open the [IdMux Wiki](https://github.com/4nass/idmux-proxy/wiki).
2. Open a page.
3. Click **Edit**.
4. Make a small change.
5. Add a short edit message.
6. Click **Save Page**.

## Edit with Git

The first Wiki page must exist before cloning it.

```text
gh auth setup-git
git clone https://github.com/4nass/idmux-proxy.wiki.git
cd idmux-proxy.wiki
git config user.name 4nass
git config user.email 4nass@users.noreply.github.com
```

Edit a Markdown page, then publish it:

```text
git add .
git commit -m "docs: update wiki"
git push origin master
```

The Wiki does not have the same pull request flow as the main repository.
Keep write access limited to trusted contributors. Use an issue or a pull
request in the main repository when a documentation change needs review.

## Writing rules

- Use simple English.
- Use short sentences.
- Add a command when a reader must run something.
- Keep links working.
- Never add secrets, cookies, tokens, keys, or private data.
