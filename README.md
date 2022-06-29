# Receptor IP Intelbras

O objetivo principal deste projeto é implementar um "Receptor IP" alternativo
para centrais de alarme Intelbras.

Um objetivo secundário do projeto é fornecer uma documentação alternativa
para os protocolos dessas centrais de alarme, no espírito "code-as-documentation".

## Estado da implementação

No momento, o receptor implementa o protocolo da forma mais correta
possível (até onde vai nosso entendimento) e interpreta todos os eventos
listados na documentação (arme, desarme, disparo de zona, fechamento de
zona, etc.), reportando mensagens legíveis.

O programa também é capaz de fazer o download de fotos de disparos do sensor
IVP-8000.

Há potencial de extensão, que reside nos ganchos. Os ganchos são scripts
invocados em disparos (`gancho_msg` e `gancho_arquivo`).
Neles, você pode adicionar código e fazer o repasse dos eventos
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

## Limitações atuais

No momento o receptor é apenas um programa de linha de comando, que 
imprime o log na saída de terminal.

O programa não tem pacote de instalação nem imagem Docker. No momento, é
necessário que o usuário possua algum conhecimento de "devops" para fazer
uso deste programa.

O Receptor não verifica a versão de Python instalada em seu sistema, mas
deve ser razoavelmente atual (mínimo absoluto 3.5, recomendado 3.8 ou acima).

Ao fazer a conexão de callback para download de fotos, o Receptor presume que
o IP da central é o mesmo da conexão principal. Isto pode não funcionar se
houver um roteador NAT no caminho (este é outro motivo pelo qual usamos VPN).

## Como rodar

Invoque o programa via linha de comando

```
./receptorip <porta> <porta central> <senha> <tamanho senha>
```

"Porta" é a porta TCP/IP em que o Receptor IP aceita conexão.
Se passado o valor 0, ouve na porta default 9009. (Pode-se usar
qualquer número de porta, desde que a respectiva configuração seja atualizada
na central.)

"Porta central" é a porta TCP/IP em que a central aceita conexão.
Se passado o valor 0, usa a porta default 9009. Esta porta é utilizada
para conexões de callback, utilizadas apenas obter fotos de um disparo de zona
do sensor IVP-8000.

"Senha" é a senha de acesso remoto (usuário 98), a mesma utilizada
para acesso à central via app AMT Mobile. Utilizada apenas para obter
fotos do sensor IVP-8000. Informe um número qualquer se não deseja fazer
download de fotos.

"Tamanho senha" é o tamanho da senha acima. Informe 4 ou 6, ou zero se
não deseja que seja feito download de fotos.

Exemplo:

```
./receptorip 9010 0 123456 6
```

Se você deseja repassar as mensagens e fotos de disparo de alarme para
outros serviços (email, SMS, WhatsApp, etc.) você deve estender os
scripts-gancho (`gancho_msg` e `gancho_arquivo`).

O receptor tenta fazer o download de fotos de disparo assim que eles ocorrem.
Se for necessário fazê-lo manualmente a posteriori, pode-se utilizar o script

```
./dlfoto <IP central> <porta central> <senha> <tam.senha> <indice> <nr.foto>
```

Os primeiros parâmetros têm o mesmo significado dos passados para o 
Receptor IP, já elencados antes. O índice da foto é informado juntamente
com a mensagem de disparo. O número da foto começa em 0. No caso do sensor
IVP-8000, duas fotos são tiradas por disparo (números de foto 0 e 1).

Exemplo de uso:

```
./dlfoto 192.168.0.16 0 123456 6 404 1
```

## Log (registro de funcionamento)

O Receptor IP grava o log no arquivo `receptorip.log`. Esse registro inclui mensagens
de disparo, e também algumas mensagens administrativas (conexão e desconexão da
central, etc.)

O arquivo é fechado a cada linha gravada, então um script periódico pode renomeá-lo
e manipulá-lo a qualquer momento, sem precisar parar o monitor. (Assim que outra
mensagem de log tiver de ser gravada, o programa criará um arquivo novo com o mesmo
nome: `receptorip.log`.)

## Ganchos de monitoramento

Um grande problema de rodar um serviço na nuvem é o monitoramento. Se o serviço
parar de funcionar, como você vai ficar sabendo?
O mesmo vale para a central de alarme: se ela ficar sem bateria, ou sem conexão,
pode demorar muito tempo até que o problema seja notado.

Para auxiliar neste mister, o Receptor IP invoca dois ganchos adicionais:
`gancho_watchdog` e `gancho_central`.

O script `gancho_watchdog` é invocado religiosamente uma vez por hora, enquanto
o Receptor IP estiver rodando.

Já o script `gancho_central` só é invocado quando nenhuma central está conectada ao
Receptor IP.

Sugerimos que o script `gancho_watchdog` seja conectado a um serviço como
(healthchecks.io)[https://healthchecks.io]. Este serviço é especializado em
"notificações negativas", ou seja, ele avisa quando uma rotina periódica falha
em bater o ponto.

Para o script `gancho_central`, sugerimos que ele envie uma notificação
ao usuário, usando o mesmo método que o script `gancho_msg`, pois a falta
de conexão da central é tão preocupante quanto um disparo de alarme.

## Roadmap

- Testes unitários e de robustez.
- Script de restart em caso de quebra

## Motivação

Num caso de uso típico, uma pessoa contrata uma empresa de segurança,
que realiza dois serviços: instala o alarme na casa do cliente, e roda
o Receptor IP -- um software desenvolvido pela Intelbras -- a fim de receber
os eventos de alarme.

Porém, existem casos em que pode ser útil usar um "Receptor IP"
alternativo, por exemplo:

a) numa área onde nenhuma empresa de segurança possa atender, porém
os eventos ainda poderiam ser reportados para uma rede de vizinhos.

b) quando for desejável armazenar e/ou tratar os eventos de alarme
de forma sistemática e automatizada, salvando dados na nuvem ou
ainda disponibilizando-os na Web.

Um caso particular do ponto acima é o disparo de sensores de movimento
capazes de tirar fotos. Um invasor diligente procurará destruir
a central para eliminar essas fotos. Um Receptor IP rodando na nuvem
garante que as fotos estarão salvas.

c) A central AMT-8000 não suporta envio de SMS no firmware mais
recente. Se SMS for absolutamente necessário, o nosso programa pode
ser usado para repassar os disparos a um serviço SMS, ou integrar
com WhatsApp/Telegram. A nuvem da Amazon possui o serviço SNS para
facilitar esse tipo de integração.

(Lembrando que uma central de alarme pode reportar eventos a dois
Receptores IP, então é possível reportar ao Receptor IP original
e ao alternativo ao mesmo tempo.)

d) uma central de alarme poderia ser usada em projetos IoT não 
necessariamente relacionados com segurança patrimonial. É um
hardware barato, de boa qualidade e fácil de encontrar.

e) o Receptor IP da Intelbras é um software Windows, feito para
empresas de monitoramento que acompanham inúmeros clientes ao
mesmo tempo. A nossa alternativa rodaria facilmente num Raspberry Pi Zero.

Uma última motivação para este projeto, mais pessoal, é conhecer mais de perto
esse ecossistema das centrais de alarme. Os protocolos são verdadeiras 
cápsulas do tempo; suas implementações possuem cacoetes dos tempos em que
eventos de alarme eram reportados por DTMF, portas seriais e modem discado.

## Onde está a documentação dos protocolos?

Os documentos do protocolo Intelbras não são públicos, por isso não
há cópias deles aqui neste repositório. Mas eles mas podem ser obtidos
facilmente acionando o suporte via WhatsApp. Instruções em
https://forum.intelbras.com.br/viewtopic.php?f=2257&t=73067

Não é preciso assinar nenhum acordo de confidencialidade
para acessar ou ler esses documentos. Portanto, fica implicitamente
permitido que este projeto (e outros) possa ser de código-fonte aberto.
(O que na prática serve como documentação alternativa do protocolo.)

Aproveitamos para agradecer ao suporte Intelbras, em particular 
Robson dos Santos, pela pronta resposta e fornecimento de informações, nas
diversas ocasiões em que precisamos de direções e esclarecimentos.
