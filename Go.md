# Receptor IP Intelbras em Go

A principal motivação de reimplementar este projeto em Go é a facilidade de instalação,
visto que executáveis Go não têm dependências externas. Basta compilar e usar. 

## Diferenças em relação à versão Python

- A versão Go não suporta download de fotos, uma vez que não temos mais o sensor
IVP 8000 Ex que possuía este recurso, e nunca fizemos uso dele na prática.

- Não suporta limitação do número de conexões ou filtar pelo endereço MAC da central,
pois também eram recursos sem valor para nosso caso de uso.

- Por consequência, o arquivo de configuração possui menos variáveis.

- Os parâmetros do utilitário `gocomandar`, análogo à versão Python `comandar`, são
fornecidos em ordem ligeiramente diferente, e mais lógica.

Os recursos inexistentes na versão Go podem ser implementados no futuro, se houver interesse.

## Como rodar

Construa o programa `goreceptor` usando o toolchain do Go.

Invoque o programa via linha de comando

```
goreceptor <arquivo de configuração>
```

Exemplo de uso:

```
goreceptor config.cfg
```

## Arquivo de configuração

O arquivo de configuração ``config.cfg`` é fornecido como modelo. Os parâmetros
contidos dele, e relevantes para a implementação Go, são os seguintes:

``addr`` - interface de rede em que o Receptor IP aceitará conexões.
Deve ser um endereço IPv4 válido. Use o endereço `0.0.0.0` se a interface
é indiferente, ou se não tem certeza do endereço correto.

Se não fornecido, o default é 0.0.0.0.

``port`` - porta em que o Receptor IP aceitará conexões. Deve ser um número.
Normalmente será `9009`.

Se não fornecido, o default é 9010.

``loglevel`` - se presente, aumenta o nível de log, para fins de depuração. O mesmo efeito pode ser obtido setando a variável de ambiente LOGITBL para um valor qualquer.

``gancho_msg`` - script invocado com mensagens humanamente legíveis de eventos.

``gancho_ev`` - script invocado com dados numéricos de eventos.

``gancho_central`` - script invocado quando nenhuma central está conectada ao Receptor,
para fins de detecção de falha de rede ou central sem comunicação.

``gancho_watchdog`` - script invocado a cada 1h para fins de watchdog.

Os parâmetros de gancho são obrigatórios e devem apontar para scripts vazios, porém executáveis e válidos, se não forem
relevantes para seu caso de uso.

## Enviar comandos à central

Construa o programa `goreceptor` usando o toolchain do Go.

Invoque o programa via linha de comando

```
gocomandar <endereço:porta> <senha> <tamanho senha> <comando> [partição ou zona]
```

O programa retorna status 0 se bem-sucedido e diferente de 0 em caso de falha, o que permite seu uso em scripts shell mais
elaborados.

Comandos disponíveis: 

- `nulo` apenas autentica na central.

- `status` retorna informações sobre o status da central.

- `ativar [partição]` ativa o alarme. Se a partição não for especificada, ativa todas.

- `desativar [partição]` desativa o alarme.

- `desligarsirene [partição]` desliga a sirene. Se a partição não for especificada, desliga para todas.

- `limpardisparo` limpa registro de disparo

- `bypass [zona]` Ativa o bypass de uma zona, ou seja, deixa de monitorá-la para fins de alarme. É obrigatório especificar a zona.

- `cancelbypass [zona]` Desativa o bypass de uma zona.
