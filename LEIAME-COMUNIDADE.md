# ProtegeWX — edição da comunidade

**Para a comunidade mais guerreira do mundo: programadores.** 💪🛡️

Se você tem licença perpétua de WINDEV / WEBDEV / WINDEV Mobile presa a um dongle
Sentinel HL, e quer que esse dongle pare de conversar com o mundo sem ter que
desligar a máquina da internet, este programa é para você.

Ele não quebra proteção, não emula chave, não altera licença. Ele faz uma coisa
só, à luz do dia: **fecha a comunicação de rede do componente de licenciamento e
dos atualizadores, mostrando o comando exato antes de rodar e deixando o botão de
desfazer do lado.**

---

## Antes de tudo: entenda o que você tem

Abra `http://127.0.0.1:1947` e veja o tipo da sua chave.

Se ela for **Sentinel HL** com `locked = 1` e `cloud_based = 0`, a licença está
gravada dentro do chip do dongle. **Não existe kill-switch pela internet para esse
tipo de chave.** Ninguém "desliga" seu dongle remotamente.

O caminho real de desativação é outro: alguém aplicar um **arquivo V2C localmente
na sua máquina**, através do RUS, do `haspdinst.exe` ou do botão de atualização no
painel do Sentinel. É esse caminho que o grupo D fecha.

Então o medo faz sentido, mas o vetor não é o que parece. Proteja o certo.

---

## O que o programa faz

Quatro grupos de ações, todas reversíveis, nenhuma escrevendo dentro do dongle.

### Grupo A — Sentinel License Manager
Faz o gerenciador de licenças atender só a sua máquina.

| | |
|---|---|
| A1 | escutar a porta 1947 apenas em `127.0.0.1` |
| A2 | recusar clientes remotos |
| A3 | parar de procurar License Managers na rede |
| A4 | desligar o broadcast UDP |
| A5 | exigir senha no Admin Control Center |
| A6 | ligar registro de acessos |

**A5 merece atenção — leia isto.**

O **Admin Control Center (ACC)** é o painel de administração que vem junto com o
driver do dongle, feito pela Thales. Não tem nada a ver com o ProtegeWX: já está
na sua máquina desde que você plugou a chave. Abra `http://127.0.0.1:1947` no
navegador e veja.

**Esse painel abre sem pedir senha nenhuma.** É o padrão de fábrica. E dentro
dele existe a opção de *desabilitar uma chave* — poucos cliques e a licença para
de funcionar.

Quer dizer: qualquer um que sente no seu computador — técnico em visita,
funcionário, quem pegou a máquina emprestada — derruba sua licença sem saber
senha alguma.

Repare na diferença de cobertura:

| Ameaça | O que protege |
|---|---|
| Alguém de **outro PC da rede** abre seu ACC | A1, A2, B1, B2 (tiram a 1947 da rede) |
| Alguém **na sua própria máquina** abre o ACC | **só a A5** |

O firewall não alcança esse segundo caso, porque o acesso vem de dentro. Se for
aplicar uma única coisa desta lista inteira, aplique a A5.

**Esqueceu a senha?** Não perde nada e as licenças não são afetadas:

```
del "C:\Program Files (x86)\Common Files\Aladdin Shared\HASP\hasplm.ini"
sc stop hasplms
sc start hasplms
```

O painel volta a abrir sem senha e você configura de novo.

### Grupo B — Firewall do Windows
| | |
|---|---|
| B1 | desabilita as regras que abrem a 1947 para a rede |
| B2 | bloqueia entrada na 1947 (TCP e UDP) |
| B3 | bloqueia saída do `hasplms.exe` e `hasplmv.exe` |
| B4 | bloqueia saída para a porta 1947 de qualquer host |
| B5 | bloqueia saída dos atualizadores da PC SOFT |

> **Isso não quebra o dongle.** O Windows não filtra tráfego de loopback, e a API
> do WinDev fala com o serviço em `127.0.0.1:1947`. Bloqueia-se o que sairia da
> máquina, não o que fica dentro dela.

### Grupo C — hosts
Aponta os domínios de telemetria e atualização para o vazio.

Camada de **reforço apenas**, e o painel diz isso na cara: o arquivo hosts não
aceita curinga, não impede conexão por IP direto e é ignorado por navegador com
DNS-over-HTTPS. Quem protege de verdade é o firewall.

O bloqueio de `doc.pcsoft.fr` e do fórum vem **desmarcado** — quase todo mundo usa
a documentação online no dia a dia.

### Grupo D — proteção contra V2C
| | |
|---|---|
| D1 | grava um retrato das suas licenças (o registro de referência) |
| D2 | guarda uma cópia do runtime Sentinel para restauração |
| D3 | nega execução do `AutomaticUpdate.exe` e do `haspdinst.exe` |
| D4 | monitor diário que avisa se algo mudar nas licenças |

**D1 é o mais importante de todos e não altera nada no sistema.** Ele só registra
o que suas chaves contêm hoje: tipo, firmware, features, se a licença é perpétua.
Se um dia algo mudar, você tem com que comparar. Rode isso hoje, mesmo que não vá
aplicar mais nada.

---

## Como usar

```
protegewx.exe                 abre o painel no navegador
protegewx.exe --status        mostra o estado de tudo, sem alterar nada
protegewx.exe --check         compara suas licenças com o registro de referência
protegewx.exe --aplicar A1,B2 aplica ações específicas
protegewx.exe --reverter B5   reverte ações específicas
protegewx.exe --revert-all    desfaz tudo
```

Precisa de **prompt como administrador** (firewall e arquivos em Program Files).
O painel pede elevação sozinho.

### Ordem sugerida

1. `--status` para ver como sua máquina está hoje
2. Aplicar **D1 e D2** — registro e backup antes de mexer em qualquer coisa
3. Um grupo por vez, **abrindo o WINDEV entre cada um** para confirmar que a
   licença continua sendo enxergada
4. `--check` de vez em quando

### Um aviso que vale mais que o resto

**O grupo A reinicia o serviço `hasplms`.** Se você estiver com o WINDEV aberto,
as sessões de licença caem e você pode perder alteração não salva.
**Feche as IDEs antes de aplicar o grupo A.**

---

## Privacidade

Nesta edição os números de série dos dongles aparecem mascarados na tela
(`15••••••••`): apenas os dois primeiros dígitos, o suficiente para você
distinguir uma chave da outra. A quantidade de pontos é fixa e não entrega nem
quantos dígitos o número tem. Assim dá para tirar print e pedir ajuda num grupo ou
fórum sem expor suas chaves.

O arquivo `baseline/chaves-baseline.json` guarda os dados completos, porque é ele
que serve de prova do conteúdo original das suas licenças. **Não publique esse
arquivo.**

---

## Se algo der errado

Está tudo registrado em `logs\protegewx.log` — cada comando executado, o código de
saída e quanto tempo levou.

Para voltar a máquina ao estado original:

```
protegewx.exe --revert-all
```

Se nem isso resolver, o desfazer manual é curto:

```
del "C:\Program Files (x86)\Common Files\Aladdin Shared\HASP\hasplm.ini"
sc stop hasplms && sc start hasplms
netsh advfirewall firewall delete rule name=all program=any    (só as que começam com PROTEGEWX)
```
e remova do `hosts` o bloco entre `# >>> PROTEGEWX >>>` e `# <<< PROTEGEWX <<<`.

---

## Limites — o que este programa não faz

Ser honesto sobre isso importa mais do que parecer poderoso:

- **Não impede** quem tem acesso físico e administrativo à sua máquina de aplicar
  um V2C. D3 dificulta, não torna impossível.
- **Não recupera** uma licença já desativada.
- **Não gera C2V.** Em chaves HL travadas o próprio ACC reporta `c2v=0`: só o
  fabricante, com a Vendor Key, consegue emitir. Não é limitação do programa, é
  característica da chave.
- **Bloquear atualizações significa não receber service packs.** Isso é
  intencional, mas é uma troca — decida sabendo.
- Se sua licença exigir ativação online periódica (não é o caso das HL perpétuas),
  isolar a rede vai quebrá-la.

---

## Aviso legal

Bloquear tráfego de saída da sua própria máquina é ação legítima de quem é dono do
equipamento. Ainda assim, verifique seu contrato: deixar de atualizar pode afetar
cláusulas de manutenção e suporte. Este programa é fornecido **sem garantia** —
leia o que cada ação faz antes de aplicar, e comece pelo `--status`.

Bom código a todos. 🚀
