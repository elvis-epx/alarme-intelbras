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
IVP-8000 Pet Cam.

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

## Limitações atuais

No momento o receptor é apenas um programa de linha de comando.
Não há pacote pronto de instalação nem imagem Docker. É
necessário que o usuário possua algum conhecimento de "devops"
para fazer uso deste programa.

O Receptor não verifica a versão de Python instalada em seu sistema, mas
deve ser razoavelmente atual (mínimo absoluto 3.5, recomendado 3.8 ou acima).

O programa é voltado para o nosso caso de uso, onde uma (ou poucas) centrais
conectam-se a ele. Certamente o código pode ser adaptado para uso profissional
(digamos, para uma empresa de monitoramento), com centenas ou milhares de
centrais conectadas, com o log e os eventos de cada uma sendo tratados de forma
independente, etc. mas não é nossa prioridade.

## Como rodar

Invoque o programa via linha de comando

```
./receptorip <arquivo de configuração>
```

Exemplo de uso:

```
./receptorip config.cfg
```

## Arquivo de configuração

O arquivo de configuração ``config.cfg`` é fornecido como modelo. Os parâmetros
contidos dele são os seguintes:

``addr`` - interface de rede em que o Receptor IP aceitará conexões.
Deve ser um endereço IPv4 válido. Use o endereço `0.0.0.0` se a interface
é indiferente, ou se não tem certeza do endereço correto.

``port`` - porta em que o Receptor IP aceitará conexões. Deve ser um número.
Normalmente será `9009`.

``caddr`` - endereço IP da central de alarme. Deve ser um endereço ou hostname válido,
ou então `auto` para detecção automática.

``cport`` - porta da central de alarme. Deve ser um número. Normalmente será 9009.

``senha`` - senha de acesso remoto da central (usuário 98), a mesma utilizada
para acesso via app AMT Mobile. Deve ser um número.

``tamanho`` - tamanho da senha acima. Deve ser igual a `4` ou `6`.

``folder_dlfoto`` - pasta em que serão gravadas as fotos obtidas de sensores IVP-8000 Pet Cam.

``centrais`` - expressão regular que determina os IDs das centrais aceitas para conexão.

``maxconn`` - número máximo de centrais conectadas e identificadas. Uma vez atingido esse número, conexões novas não são tratadas e acabam fechando por timeout.

``logfile`` - arquivo de log (que também é ecoado no stdout). Informar ``None`` se não quiser que o log seja gravado em um arquivo.

``gancho_msg`` - script invocado com mensagens humanamente legíveis de eventos.

``gancho_ev`` - script invocado com dados numéricos de eventos.

``gancho_arquivo`` - script invocado quando uma foto é obtida do IVP-8000 Pet Cam.

``gancho_central`` - script invocado quando nenhuma central está conectada ao programa, para fins de detecção de falha de rede ou central sem comunicação.

``gancho_watchdog`` - script invocado a cada 1h para fins de watchdog.

Todos os parâmetros são obrigatórios e devem ser sintaticamente corretos, mesmo que não sejam utilizados no seu caso.

Os parâmetros `caddr`, `cport`, `senha`, `tamanho` e `folder_dlfoto` são relevantes apenas para obter fotos 
capturadas pelos sensores IVP-8000 Pet Cam. Se você não possui esse sensor, os valores não importam. Se deseja que
o receptor IP não obtenha as fotos, informe uma senha incorreta.

## Mais sobre autenticação da central

Exemplos de expressões regulares válidas para ``centrais``:

``.*`` qualquer ID de central

``aa:bb:cc`` apenas a central com ID `aa:bb:cc`. (O ID da central é um pedaço de endereço MAC, com dígitos hexa minúsculos.)

``aa:bb:(cd|ef)`` aceita as centrais `aa:bb:cd` e `aa:bb:ef`.

``aa:bb:cd|aa:bb:ef`` idem.

Utilize o script ``testa_re`` para testar sua expressão e ver se dá match :)

As configurações ``centrais`` e ``maxconn`` são um mecanismo básico
de filtragem contra conexões espúrias, para um Receptor IP rodando na nuvem e exposto
à Internet.

Porém, é totalmente desaconselhado contar apenas com isso
para sua segurança! O ideal é usar VPN. Uma solução temporária razoável seria
filtrar pela faixa de IPs do provedor que fornece conectividade à central de alarme.

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

## Download de fotos versus NAT

Quando o Receptor IP recebe uma conexão da central de alarme, o endereço IP
de origem é anotado. Se o parâmetro ``caddr`` for igual a ``auto``, esse
mesmo endereço é utilizado para fazer o callback à central na hora de obter as fotos
do sensor IVP 8000 Pet Cam.

Porém, isto não funcionará se o Receptor IP e a central estiverem em lados opostos
de um roteador NAT e/ou de um provedor CGNAT. Será preciso usar alguma técnica para
"furar o bloqueio", seja VPN, IP quente fixo, NAT reverso, etc.  O parâmetro ``caddr``
deve ser preenchido com o endereço IP ou hostname através do qual se possa conectar à central.

Note que expor sua central à conexões vindas da Internet é um furo de segurança.
Alguma providência adicional (VPN, firewall) deve ser adotada.

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

## Supervisão e deploy

O aplicativo `receptorip` é construído e testado para ser robusto.
Porém, devido a algum imprevisto, ou mesmo algum bug, ele pode parar
inesperadamente. Alguma espécie de supervisor deve ser empregado para
reiniciar o Receptor IP quando isto acontecer, de preferência notificando
por e-mail para que a causa-raiz seja descoberta e consertada.

Como somos da velha guarda, não somos muito fãs de Docker. Se serve de sugestão,
a forma que nós mesmos rodamos nosso Receptor IP na nuvem é a seguinte:

- Usamos o [PyInstaller](https://pyinstaller.org/en/stable/) para transformar os scripts ``receptorip`` e (se necessário) o ``comandar`` em executáveis de arquivo único, que podem ser copiados para ``/usr/local/bin`` e usados como se fossem utilitários Unix comuns.

- Posicionar o arquivo de configuração e os scripts de gancho numa pasta em ``/etc``.

- Usar o `systemd` como meio de iniciar e supervisionar o serviço `receptorip`.

## Enviar comandos à central

O Receptor IP apenas recebe eventos da central. Em algumas situações, pode ser interessante
enviar comandos à central. Por exemplo, ativar ou desativar o alarme de forma integrada com a
automação residencial.

Para esse fim, oferecemos o script `comandar`.

- Este script funciona de forma independente do Receptor IP, nem faz uso do arquivo de configuração. Você não precisa ter um Receptor IP funcionando para fazer uso deste script.

- Todos os parâmetros (endereço IP da central, senha) devem ser passados como parâmetros de linha de comando.

- Rode o script sem parâmetros para obter um texto de ajuda, bem como a lista de comandos suportados.

- O utilitário ainda está em desenvolvimento, ou seja, ainda estamos adicionando comandos e recursos.

- O protocolo ISECNet v2, utilizado nesses comandos, é o mesmo utilizado para download de fotos do sensor IVP 8000 Pet Cam, e acreditamos que seja implementado apenas pela central AMT 8000.

(Nota: O ISECNet também pode trafegar de forma multiplexada através da conexão entre a central e
o Receptor IP, evitando a necessidade de abrir nova conexão TCP/IP com a central. Não
implementamos essa modalidade, mas ela existe.)

### Download manual de fotos

O receptor tenta fazer o download de fotos de disparo assim que eles ocorrem.
Mas é possível também fazê-lo a posteriori usando o script `comandar` abordado logo acima.

Para fazer download de uma foto, devem ser informados dois números: índice e número
da foto. O índice da foto é informado na mensagem de disparo. O número da foto começa
em 0. No caso do sensor IVP-8000 Pet Cam, duas fotos são tiradas por disparo (números
de foto 0 e 1).

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
