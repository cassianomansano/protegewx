/* ProtegeWX — painel local.
   O princípio do painel: nada é executado antes de o usuário ver o comando exato,
   o risco e a forma de reverter. Toda tela é montada a partir do catálogo de ações
   que o programa expõe em /api/estado. */

'use strict';

let estado = { grupos: [], acoes: [], diagnostico: {}, chaves: [], features: [] };

const $  = (s) => document.querySelector(s);
const el = (tag, cls, txt) => {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (txt !== undefined) e.textContent = txt;
  return e;
};

/* ------------------------------------------------------------------ dados */

async function carregar() {
  $('#diag-corpo').className = 'carregando';
  $('#diag-corpo').textContent = 'lendo o sistema…';
  try {
    const r = await fetch('/api/estado');
    if (!r.ok) throw new Error('HTTP ' + r.status);
    estado = await r.json();
    estado.acoes = estado.acoes || [];
    desenharDiagnostico();
    desenharLicencas();
    desenharGrupos();
    atualizarSelecao();
  } catch (e) {
    $('#diag-corpo').className = '';
    $('#diag-corpo').innerHTML = '<div class="msg erro">Não foi possível ler o estado: ' + escapar(e.message) + '</div>';
  }
}

const escapar = (s) => String(s ?? '').replace(/[&<>"']/g,
  (c) => ({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;' }[c]));

/* ------------------------------------------------------------------ diagnóstico */

function desenharDiagnostico() {
  const d = estado.diagnostico || {};
  const cfg = d.config || {};
  const alvo = $('#diag-corpo');
  alvo.className = '';
  alvo.innerHTML = '';

  const grade = el('div', 'grade');

  const exposta = d.portaExposta === true;
  grade.appendChild(cartao('Porta 1947',
    exposta ? 'exposta na rede' : 'somente local',
    exposta ? 'ruim' : 'bom',
    exposta ? 'escutando em ' + (d.enderecosExpostos || []).join(', ') : 'apenas 127.0.0.1'));

  grade.appendChild(cartao('Clientes remotos',
    cfg.aceitaRemoto ? 'aceitos' : 'recusados',
    cfg.aceitaRemoto ? 'ruim' : 'bom'));

  grade.appendChild(cartao('Busca na rede',
    cfg.procuraRemoto ? 'ligada' : 'desligada',
    cfg.procuraRemoto ? 'aten' : 'bom'));

  grade.appendChild(cartao('Broadcast UDP',
    cfg.broadcast ? 'ligado' : 'desligado',
    cfg.broadcast ? 'aten' : 'bom'));

  grade.appendChild(cartao('Senha do painel Sentinel',
    cfg.accComSenha ? 'exigida' : 'nenhuma',
    cfg.accComSenha ? 'bom' : 'ruim'));

  const n = (d.regrasCriadas || []).length;
  grade.appendChild(cartao('Regras do ProtegeWX', n + (n === 1 ? ' regra' : ' regras'), n ? 'bom' : 'aten'));

  alvo.appendChild(grade);
}

function cartao(rotulo, valor, classe, detalhe) {
  const c = el('div', 'item');
  c.appendChild(el('div', 'rot', rotulo));
  c.appendChild(el('div', 'val ' + (classe || ''), valor));
  if (detalhe) c.appendChild(el('div', 'sutil', detalhe));
  return c;
}

/* ------------------------------------------------------------------ licenças */

function desenharLicencas() {
  const alvo = $('#lic-corpo');
  alvo.className = '';
  const chaves = estado.chaves || [];
  const feats = estado.features || [];

  if (!chaves.length) {
    alvo.innerHTML = '<div class="msg erro">Nenhum dongle foi encontrado pelo gerenciador de licenças. ' +
                     'Verifique se as chaves estão conectadas e se o serviço hasplms está em execução.</div>';
    return;
  }

  const t = el('table', 'lic');
  t.innerHTML = '<thead><tr><th>Chave</th><th>Tipo</th><th>Firmware</th>' +
                '<th>Licença</th><th>Situação</th></tr></thead>';
  const tb = el('tbody');

  chaves.forEach((c) => {
    const meus = feats.filter((f) => f.haspid === c.haspid);
    const perp = meus.length && meus.every((f) => (f.lic || '').toLowerCase() === 'perpetual');
    const ruim = meus.some((f) => f.dis !== '0' || f.ex !== '0' || f.unusable !== '0') ||
                 c.key_disabled !== '0';

    const tr = el('tr');
    tr.innerHTML =
      '<td><code>' + escapar(c.haspid) + '</code></td>' +
      '<td>' + escapar(c.typ) + '</td>' +
      '<td>' + escapar(c.fw) + '</td>' +
      '<td>' + (perp ? '<span class="etq aplicado">perpétua</span>'
                     : escapar((meus[0] && meus[0].lic) || '—')) + '</td>' +
      '<td>' + (ruim ? '<span class="etq risco-alto">verificar</span>'
                     : '<span class="etq aplicado">íntegra</span>') + '</td>';
    tb.appendChild(tr);
  });

  t.appendChild(tb);
  alvo.innerHTML = '';
  alvo.appendChild(t);

  const nota = el('p', 'sutil');
  nota.style.marginTop = '12px';
  nota.textContent = 'Estas chaves são de hardware, com a licença gravada no próprio chip. ' +
    'Não existe desativação remota pela internet para esse tipo de chave: o único caminho de ' +
    'desativação é a aplicação local de um arquivo de atualização de licença, que é justamente ' +
    'o que o grupo D bloqueia.';
  alvo.appendChild(nota);
}

/* ------------------------------------------------------------------ grupos e ações */

function desenharGrupos() {
  const alvo = $('#grupos');
  alvo.innerHTML = '';

  (estado.grupos || []).forEach((g) => {
    const cab = el('div', 'grupo-cab');
    const h = el('h2');
    h.appendChild(el('span', 'selo', 'Grupo ' + g.id));
    h.appendChild(document.createTextNode(g.nome));
    cab.appendChild(h);
    cab.appendChild(el('p', null, g.resumo));
    if (g.ressalva) cab.appendChild(el('div', 'ressalva', g.ressalva));
    alvo.appendChild(cab);

    estado.acoes.filter((a) => a.grupo === g.id).forEach((a) => alvo.appendChild(desenharAcao(a)));
  });
}

function desenharAcao(a) {
  const box = el('div', 'acao' + (a.estado === 'aplicado' ? ' aplicada' : ''));
  box.id = 'acao-' + a.id;

  const chk = el('input');
  chk.type = 'checkbox';
  chk.dataset.id = a.id;
  chk.disabled = a.estado === 'indisponivel';
  // pré-marca o que é padrão e ainda não está aplicado
  chk.checked = a.padrao && a.estado !== 'aplicado' && a.estado !== 'indisponivel';
  chk.addEventListener('change', atualizarSelecao);
  box.appendChild(chk);

  const corpo = el('div');

  const cab = el('div', 'acao-cab');
  cab.appendChild(el('span', 'acao-id', a.id));
  cab.appendChild(el('h3', null, a.titulo));
  cab.appendChild(el('span', 'etq ' + a.estado, rotuloEstado(a.estado)));
  cab.appendChild(el('span', 'etq risco-' + a.risco, 'risco ' + a.risco));
  corpo.appendChild(cab);

  corpo.appendChild(el('p', null, a.oQueFaz));

  const p1 = el('p', 'porque');
  p1.innerHTML = '<b>Por que:</b> ' + escapar(a.porQue);
  corpo.appendChild(p1);

  const p2 = el('p', 'reverte');
  p2.innerHTML = '<b>Como desfazer:</b> ' + escapar(a.reverte);
  corpo.appendChild(p2);

  const det = el('details', 'cmds');
  det.appendChild(el('summary', null, 'Ver o que será executado (' + (a.comandos || []).length + ')'));
  det.appendChild(el('pre', null, (a.comandos || []).join('\n')));
  corpo.appendChild(det);

  box.appendChild(corpo);
  return box;
}

function rotuloEstado(e) {
  return { aplicado: 'aplicado', 'nao-aplicado': 'não aplicado',
           parcial: 'parcial', indisponivel: 'não se aplica' }[e] || e;
}

/* ------------------------------------------------------------------ seleção */

function selecionados() {
  return [...document.querySelectorAll('.acao input[type=checkbox]:checked')].map((c) => c.dataset.id);
}

function atualizarSelecao() {
  const n = selecionados().length;
  $('#resumo-sel').textContent = n === 0 ? 'nenhum item selecionado'
    : n + (n === 1 ? ' item selecionado' : ' itens selecionados');
  $('#btn-aplicar').disabled = n === 0;
  $('#btn-reverter').disabled = n === 0;
}

/* ------------------------------------------------------------------ modal */

function confirmar(titulo, montarCorpo) {
  return new Promise((resolve) => {
    $('#modal-titulo').textContent = titulo;
    const corpo = $('#modal-corpo');
    corpo.innerHTML = '';
    montarCorpo(corpo);
    $('#modal').classList.remove('oculto');

    const fim = (v) => {
      $('#modal').classList.add('oculto');
      $('#modal-sim').removeEventListener('click', sim);
      $('#modal-nao').removeEventListener('click', nao);
      resolve(v);
    };
    const sim = () => fim(true);
    const nao = () => fim(false);
    $('#modal-sim').addEventListener('click', sim);
    $('#modal-nao').addEventListener('click', nao);
  });
}

/* ------------------------------------------------------------------ execução */

async function executar(ids, reverter) {
  const acoes = ids.map((id) => estado.acoes.find((a) => a.id === id)).filter(Boolean);
  const verbo = reverter ? 'Reverter' : 'Aplicar';

  // A5 precisa da senha antes de ser aplicada
  if (!reverter && ids.includes('A5') && !estado.diagnostico.temSenhaDefinida) {
    const senha = await pedirSenha();
    if (senha === null) return;
    const r = await fetch('/api/senha', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ senha }),
    }).then((x) => x.json());
    if (!r.ok) { alert('Não foi possível guardar a senha: ' + r.erro); return; }
  }

  const ok = await confirmar(verbo + ' ' + acoes.length + (acoes.length === 1 ? ' ação' : ' ações'), (c) => {
    c.appendChild(el('p', null, 'Serão executados os comandos abaixo, nesta ordem:'));
    acoes.forEach((a) => {
      c.appendChild(el('p', null, a.id + ' — ' + a.titulo));
      c.appendChild(el('pre', null, (a.comandos || []).join('\n')));
    });
    if (!reverter) {
      const riscos = acoes.filter((a) => a.risco !== 'baixo');
      if (riscos.length) {
        const d = el('div', 'ressalva');
        d.appendChild(el('b', null, 'Itens de risco não trivial nesta seleção:'));
        const ul = el('ul');
        riscos.forEach((a) => ul.appendChild(el('li', null, a.id + ' — ' + a.titulo + ': ' + a.reverte)));
        d.appendChild(ul);
        c.appendChild(d);
      }
    }
  });
  if (!ok) return;

  const alvo = reverter ? '/api/reverter' : '/api/aplicar';
  const btns = [$('#btn-aplicar'), $('#btn-reverter'), $('#btn-reverter-tudo')];
  btns.forEach((b) => (b.disabled = true));

  try {
    const res = await fetch(alvo, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ ids }),
    }).then((r) => r.json());

    (res || []).forEach((r) => {
      const box = $('#acao-' + r.id);
      if (!box) return;
      box.querySelectorAll('.msg').forEach((m) => m.remove());
      const m = el('div', 'msg ' + (r.ok ? 'ok' : 'erro'),
        r.ok ? verbo.toLowerCase() + ' com sucesso — estado agora: ' + rotuloEstado(r.novoEstado)
             : 'não foi possível: ' + r.erro);
      box.lastElementChild.appendChild(m);
    });
  } catch (e) {
    alert('Falha na comunicação com o ProtegeWX: ' + e.message);
  } finally {
    btns.forEach((b) => (b.disabled = false));
    await carregar();
  }
}

function pedirSenha() {
  return new Promise((resolve) => {
    confirmar('Definir a senha do painel Sentinel', (c) => {
      c.appendChild(el('p', null,
        'Esta senha passará a ser exigida para abrir o Admin Control Center em http://127.0.0.1:1947.'));
      const i = document.createElement('input');
      i.type = 'password';
      i.id = 'campo-senha';
      i.placeholder = 'senha do Admin Control Center';
      c.appendChild(i);
      const n = el('p', 'sutil',
        'A senha fica registrada em estado.json, com permissão restrita, para que você não perca ' +
        'o acesso ao próprio painel. Guarde-a também no seu gerenciador de senhas.');
      n.style.marginTop = '10px';
      c.appendChild(n);
      setTimeout(() => i.focus(), 50);
    }).then((ok) => {
      const v = (document.getElementById('campo-senha') || {}).value || '';
      resolve(ok && v ? v : null);
    });
  });
}

/* ------------------------------------------------------------------ conferência */

async function conferir() {
  const alvo = $('#check-corpo');
  alvo.innerHTML = '<p class="carregando">comparando com o registro de referência…</p>';
  try {
    const r = await fetch('/api/check').then((x) => x.json());
    alvo.innerHTML = '';
    if (!r.ok) {
      alvo.innerHTML = '<div class="msg erro">' + escapar(r.erro) + '</div>';
      return;
    }
    if (!r.alertas || !r.alertas.length) {
      alvo.innerHTML = '<div class="msg ok">Nenhuma divergência. As licenças estão exatamente como no registro de referência.</div>';
      return;
    }
    r.alertas.forEach((a) => {
      const d = el('div', 'alerta ' + a.gravidade);
      d.appendChild(el('b', null, a.assunto));
      d.appendChild(document.createTextNode(a.detalhe));
      alvo.appendChild(d);
    });
  } catch (e) {
    alvo.innerHTML = '<div class="msg erro">' + escapar(e.message) + '</div>';
  }
}

/* ------------------------------------------------------------------ eventos */

$('#btn-aplicar').addEventListener('click', () => executar(selecionados(), false));
$('#btn-reverter').addEventListener('click', () => executar(selecionados(), true));
$('#btn-recarregar').addEventListener('click', carregar);
$('#btn-check').addEventListener('click', conferir);

$('#btn-reverter-tudo').addEventListener('click', async () => {
  const aplicadas = estado.acoes.filter((a) => a.estado === 'aplicado' || a.estado === 'parcial');
  if (!aplicadas.length) { alert('Não há nada aplicado para reverter.'); return; }
  await executar(aplicadas.map((a) => a.id), true);
});

carregar();
