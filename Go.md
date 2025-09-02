# Receptor IP Intelbras em Go

A principal motivação de reimplementar este projeto em Go é a facilidade de instalação,
visto que executáveis Go não têm dependências externas. Basta compilar e usar. 

## Diferenças em relação à versão Python

- A versão Go não suporta download de fotos, uma vez que não temos mais o sensor
IVP 8000 Ex que possuía este recurso, e nunca fizemos uso dele na prática.

- Não suporta limitação do número de conexões ou filtrar pelo endereço MAC da central,
pois também eram recursos sem valor para nosso caso de uso.

- Por consequência, o arquivo de configuração possui menos variáveis.

- Os parâmetros do utilitário `gocomandar`, análogo à versão Python `comandar`, devem ser
fornecidos em ordem ligeiramente diferente, e (em nossa opinião) mais lógica.

Os recursos inexistentes na versão Go podem ser portados no futuro, se houver interesse.

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

Em produção, recomenda-se usar algum supervisor como `systemd`, Docker, etc.

## Arquivo de configuração

O arquivo de configuração ``config.cfg`` é fornecido como modelo. Os parâmetros
contidos dele, e relevantes para a implementação Go, são os seguintes:

``addr`` - interface de rede em que o Receptor IP aceitará conexões.
Deve ser um endereço IPv4 válido, ou `0.0.0.0` para aceitar conexão via qualquer interface.
Se não fornecido, o default é 0.0.0.0.

``port`` - porta em que o Receptor IP aceitará conexões. Deve ser um número.
Se não fornecido, o default é 9010.

``loglevel`` - se presente, aumenta o nível de log, para fins de depuração. (O mesmo efeito pode ser obtido 
com a variável de ambiente LOGITBL.)

``gancho_msg`` - programa invocado com mensagens humanamente legíveis de eventos.

``gancho_ev`` - programa invocado com dados numéricos de eventos.

``gancho_central`` - programa invocado quando nenhuma central está conectada ao Receptor,
para fins de detecção de falha de rede ou central sem comunicação.

``gancho_watchdog`` - programa invocado a cada 1h para fins de watchdog.

Os parâmetros de gancho são obrigatórios, e os programas (ou mais provavelmente scripts) apontados por eles
devem existir e ser executáveis, mesmo que não façam nada útil.

## Enviar comandos à central

Construa o programa `goreceptor` usando o toolchain do Go.

Invoque o programa via linha de comando

```
gocomandar <endereço:porta> <senha> <tamanho senha> <comando> [partição ou zona]
```

Exemplo, com parte da saída:

```
$ gocomandar 192.168.50.12:9009 876543 6 status

*******************************************
Central AMT-8000
Versão de firmware 2.3.1
Status geral:
	 Partição(ões) armada(s)
	Zonas em alarme: Não
	Zonas canceladas: Não
	Todas zonas fechadas: Sim
	Sirene: Não
	Problemas: Não
Partição 00:
	Stay: Não
	Delay de saída: Não
	Pronto para armar: Sim
	Alame ocorreu: Não
	Em alarme: Não
	Armado modo stay: Não
	Armado: Não
Partição 01:
	Stay: Não
	Delay de saída: Não
	Pronto para armar: Sim
	Alame ocorreu: Não
	Em alarme: Não
	Armado modo stay: Não
	Armado: Sim
Partição 02:
	Stay: Não
	Delay de saída: Não
	Pronto para armar: Sim
	Alame ocorreu: Não
	Em alarme: Não
	Armado modo stay: Não
	Armado: Não
Zonas abertas:
Zonas em alarme:
Zonas em bypass:
Sirenes ligadas:
*******************************************

Sucesso
```

O programa `gocomandar` retorna status 0 se bem-sucedido e diferente de 0 em caso de falha, o que permite a integração com scripts
shell e rotinas de automação.

Comandos disponíveis: 

- `nulo` apenas autentica na central.

- `status` retorna informações sobre o status da central.

- `ativar [partição]` ativa o alarme. Se a partição não for especificada, ativa todas.

- `desativar [partição]` desativa o alarme.

- `desligarsirene [partição]` desliga a sirene. Se a partição não for especificada, desliga para todas.

- `limpardisparo` limpa registro de disparo

- `bypass [zona]` Ativa o bypass de uma zona, ou seja, deixa de monitorá-la para fins de alarme. É obrigatório especificar a zona.

- `cancelbypass [zona]` Desativa o bypass de uma zona.
