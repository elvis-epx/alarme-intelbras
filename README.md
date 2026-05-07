# Receptor IP Intelbras

O objetivo principal deste projeto é implementar um "Receptor IP" alternativo
para centrais de alarme Intelbras.

Um objetivo secundário do projeto é fornecer uma documentação alternativa
para os protocolos dessas centrais de alarme, no espírito "code-as-documentation".

# Por que a linguagem Go?

A principal motivação de reimplementar este projeto em Go é a facilidade de instalação
e distribuição, visto que executáveis Go não têm dependências externas.

Informalmente, oferecemos [versões pré-compiladas dos programas](builds/) para quem não quer lidar com toolchains e
instruções de compilação.

Quem quiser fazer o build por conta própria, pode consultar o Makefile.

E quem estiver interessado no código antigo em Python, consulte 
[o branch "python"](https://github.com/elvis-epx/alarme-intelbras/tree/python) do
repositório.

## Estado da implementação

No momento, o receptor implementa o protocolo da forma mais correta
possível (até onde vai nosso entendimento) e interpreta todos os eventos
listados na documentação (arme, desarme, disparo de zona, fechamento de
zona, etc.), reportando mensagens legíveis.

Há potencial de extensão, que reside nos ganchos. Os ganchos são scripts
invocados em disparos (`gancho_msg`, `gancho_ev`, `gancho_arquivo` e outros).
Neles, você pode adicionar código e fazer o compartilhamento dos eventos
via e-mail, SMS, Telegram, PushOver, etc.

## Plataforma e hardware de alarme

O programa é desenvolvido e testado nos sistemas operacionais Linux
e macOS, com ênfase maior no Linux, pois nosso caso de uso é um
"Receptor IP" rodando na nuvem.

Em nosso projeto, fazemos testes apenas com a central AMT-8000, que
é a que possuímos, embora o
protocolo pareça ser o mesmo para todas as centrais monitoradas via Internet.

Uma vez que o protocolo não é criptografado, utilizamos VPN entre alarme e
Receptor, e recomendamos que você faça o mesmo!

## Como rodar

Construa o programa `goreceptor` usando o toolchain do Go, ou obtenha uma versão pré-compilada 
[aqui.](builds/)

Invoque o programa via linha de comando

```
goreceptor <arquivo de configuração>
```

Exemplo de uso:

```
goreceptor config.cfg
```

Em produção, recomenda-se usar algum supervisor como `systemd`, Docker, etc.

## Limitações atuais

No momento o receptor é apenas um programa de linha de comando.
Não há pacote Debian ou imagem Docker pronto para uso. É
necessário que o usuário possua algum conhecimento de "devops"
para fazer uso deste programa.

O programa é voltado para o nosso caso de uso, onde uma (ou poucas) centrais
conectam-se a ele. Certamente o código pode ser adaptado para uso profissional
(digamos, para uma empresa de monitoramento), com centenas ou milhares de
centrais conectadas, com o log e os eventos de cada uma sendo tratados de forma
independente, etc. mas não é nossa prioridade.

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

## Mais sobre o número máximo de conexões

Aparentemente, centrais com firmware versão anterior a 2.0.6 têm bugs relacionados
ao Receptor IP. Problemas observados:

a) tenta conectar com os Receptores 1 e 2, mesmo que o 2 esteja desativado.

b) tenta conectar com o IP do Receptor 1 e também com o nome DNS do Receptor 1,
mesmo que a configuração indique conectar apenas pelo IP ou apenas pelo nome.

Esses bugs podem fazer a central criar várias conexões paralelas com o Receptor IP,
gerando duplicação de eventos.

Uma solução de contorno é cadastrar o Receptor 2 com um IP/porta inválido, mas
aí a central reportará falhas de entrega de evento ao Receptor 1 (de vez em quando),
por não conseguir falar com o Receptor 2.

Mencionamos estes problemas nesta seção porque, se você configurar o Receptor IP
para aceitar apenas 1 conexão, a central pode ficar tentando criar uma segunda
conexão continuamente, o que pode ser confuso quando a não se conhece a causa.

De todo modo, a solução ideal é fazer o update de firmware para 2.0.6, o que
pode inclusive ser comandado pelo app de configuração AMT Remoto Mobile.

## Scripts de gancho

Se você deseja compartilhar as mensagens e fotos de disparo através
de algum serviço (email, SMS, WhatsApp, etc.) faça-o através dos
scripts-gancho (`gancho_msg`, `gancho_ev` e `gancho_arquivo`).

A localização dos diversos scripts de gancho é especificada no arquivo de configuração.
Todos devem existir e ser executáveis; use scripts inócuos para ganchos que você
não precise utilizar.

O script `gancho_msg` recebe e encaminha as mensagens de eventos.

O script `gancho_ev` recebe e encaminha os códigos numéricos de eventos. Para
conhecer os códigos, consulte a documentação da Intelbras ou o início do arquivo
`alarmeitbl/tratador.py`.

O script `gancho_arquivo` recebe e encaminha arquivos, mais especificamente
as fotos capturadas pelo sensor IVP 8000 Pet Cam.

## Ganchos de monitoramento

Um grande problema de rodar um serviço na nuvem é o monitoramento. Se o serviço
parar de funcionar, como você vai ficar sabendo?
O mesmo vale para a central de alarme: se ela ficar sem bateria, ou sem conexão,
pode demorar muito tempo até que o problema seja notado.

Para auxiliar neste mister, o Receptor IP invoca dois ganchos adicionais:
`gancho_watchdog` e `gancho_central`.

O script `gancho_watchdog` é invocado religiosamente uma vez por hora, enquanto
o Receptor IP estiver rodando.

Já o script `gancho_central` é invocado quando nenhuma central está conectada ao
Receptor IP (e quando o problema foi resolvido). O script recebe um parâmetro
igual a 1 quando o problema é detectado, e 0 quando o problema é resolvido.

Sugerimos que o script `gancho_watchdog` seja conectado a um serviço como
[healthchecks.io](https://healthchecks.io). Este serviço é especializado em
"notificações negativas", ou seja, ele avisa quando uma rotina periódica falha
em bater o ponto.

Para o script `gancho_central`, sugerimos que ele envie uma notificação
ao usuário, usando o mesmo método que o script `gancho_msg`, pois a falta
de conexão da central é tão preocupante quanto um disparo de alarme.

## Enviar comandos à central

Construa o programa `gocomandar` usando o toolchain do Go, ou obtenha uma versão pré-compilada 
[aqui.](builds/)


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

## Motivação

Num caso de uso típico, uma pessoa contrata uma empresa de segurança,
que realiza dois serviços: instala o alarme na casa do cliente, e roda
o Receptor IP -- um software desenvolvido pela Intelbras -- a fim de receber
os eventos de alarme.

Porém, existem casos em que pode ser útil usar um "Receptor IP"
alternativo:

a) numa área onde nenhuma empresa de segurança possa atender, porém
os eventos ainda poderiam ser compartilhados numa rede de vizinhos.

b) quando for desejável armazenar e/ou tratar os eventos de alarme
de forma sistemática e automatizada, salvando dados na nuvem ou
ainda disponibilizando-os na Web.

Um caso particular do ponto acima é o disparo de sensores de movimento
capazes de tirar fotos. Um invasor diligente procurará destruir
a central para eliminar essas fotos. Um Receptor IP rodando na nuvem
garante que as fotos estarão salvas.

c) A central AMT-8000 não suporta envio de SMS no firmware mais
recente. Nosso programa poderia ser usado para repassar os disparos
a um serviço SMS. Existem muitos serviços SMS pagos, e a própria Amazon
oferece o serviço SNS.

(Lembrando que uma central de alarme pode reportar eventos a dois
Receptores IP, então é possível reportar a uma central de monitoramento
e também ao receptor alternativo ao mesmo tempo.)

d) uma central de alarme poderia ser usada em projetos IoT não 
necessariamente relacionados com segurança patrimonial. É um
hardware barato, de boa qualidade e fácil de encontrar.

e) o Receptor IP da Intelbras é um software Windows, feito para
empresas de monitoramento que acompanham inúmeros clientes ao
mesmo tempo. A nossa alternativa funciona no Linux, viabilizando seu
uso na nuvem e também em SBCs tipo Raspberry Pi.

f) integração com automação residencial (Home Assistant e outros).

Uma última motivação para este projeto, mais pessoal, é conhecer mais de perto
esse ecossistema das centrais de alarme. Os protocolos são verdadeiras 
cápsulas do tempo; suas implementações possuem cacoetes dos tempos em que
eventos de alarme eram reportados por DTMF, portas seriais e modem discado.

## Onde está a documentação dos protocolos?

Os documentos do protocolo Intelbras não são públicos, por isso não
há cópias deles aqui neste repositório. Mas eles podem ser obtidos
facilmente acionando o suporte via WhatsApp. Instruções em
https://forum.intelbras.com.br/viewtopic.php?f=2257&t=73067

Não é preciso assinar nenhum acordo de confidencialidade
para acessar ou ler esses documentos. Portanto, fica implicitamente
permitido que este projeto (e outros) possa ser de código-fonte aberto.
(O que na prática serve como documentação alternativa do protocolo.)

Aproveitamos para agradecer ao suporte Intelbras, em particular 
Robson dos Santos, pela pronta resposta e fornecimento de informações, nas
diversas ocasiões em que precisamos de direções e esclarecimentos.
