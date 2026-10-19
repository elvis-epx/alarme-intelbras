# Teste contra central real

Além de `make test`, é possível validar o sistema contra uma central Intelbras
real usando o `gocomandar`:

    go run ./cmd/gocomandar <endereço:porta> <senha> <tamanho senha> <comando>

Endereço, senha e tamanho da senha (4 ou 6) ficam em `CLAUDE.local.md`, que não
é versionado. Se esse arquivo não existir, não tente o teste ao vivo.

Testes de fumaça (somente leitura, podem ser executados sem perguntar):

- `nulo`: autentica e encerra a sessão
- `status`: solicita o status da central

Saída esperada: `Sucesso` (código de saída 0). `Fracasso` sai com código 2.

`LOGITBL=1` mostra o diálogo com a central em stderr, mas o pacote de
autenticação expõe a senha em hexadecimal. Use apenas para diagnosticar falhas,
e só depois de perguntar.

Comandos que alteram o estado da central (`ativar`, `desativar`,
`desligarsirene`, `limpardisparo`, `bypass`, `cancelbypass`) só devem ser
executados quando o usuário pedir explicitamente.

Não repita a senha em mensagens, commits ou arquivos versionados.
