"use strict";

// Every piece of text coming from the server enters the page through
// textContent, never innerHTML: entry text is typed by the user, and innerHTML
// would open an XSS hole. The CSP (default-src 'self') is the second barrier,
// not the first.
//
// UI strings are in Portuguese on purpose: the product is for Brazilian users.

const brl = new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" });
const dataBR = new Intl.DateTimeFormat("pt-BR", { day: "2-digit", month: "2-digit", timeZone: "UTC" });

const CATEGORIAS = {
  alimentacao: "Alimentação", mercado: "Mercado", transporte: "Transporte",
  casa: "Casa", saude: "Saúde", lazer: "Lazer", educacao: "Educação",
  assinaturas: "Assinaturas", salario: "Salário", freelancer: "Freelancer",
  investimentos: "Investimentos", outros: "Outros",
};
const FORMAS = { dinheiro: "dinheiro", pix: "pix", debito: "débito", credito: "crédito" };

const $ = (id) => document.getElementById(id);
const reais = (centavos) => brl.format(centavos / 100);

async function api(metodo, caminho, corpo) {
  const opcoes = { method: metodo, headers: {} };
  if (corpo !== undefined) {
    opcoes.headers["Content-Type"] = "application/json";
    opcoes.body = JSON.stringify(corpo);
  }
  const resp = await fetch(caminho, opcoes);
  if (resp.status === 204) return null;
  const dados = await resp.json().catch(() => ({}));
  if (!resp.ok) throw new Error(dados.erro || `Erro ${resp.status}`);
  return dados;
}

function hojeISO() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

// "180,00" from cents: the format the user types, for pre-filling the paid amount.
const valorDigitavel = (centavos) => (centavos / 100).toFixed(2).replace(".", ",");

function mesAtual() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

async function carregarResumo() {
  const r = await api("GET", `/api/resumo?mes=${encodeURIComponent($("mes").value)}`);
  $("receitas").textContent = reais(r.receitas);
  $("despesas").textContent = reais(r.despesas);
  $("economia").textContent = reais(r.economia);
  $("a-pagar").textContent = reais(r.a_pagar);
  const v = r.vencidas;
  $("vencidas").hidden = v.quantidade === 0;
  $("vencidas").textContent = v.quantidade === 1
    ? `1 conta vencida: ${reais(v.centavos)}`
    : `${v.quantidade} contas vencidas: ${reais(v.centavos)}`;
  mostrarCategorias(r.categorias, r.despesas);
}

// Each category with its amount, its share of the month and a bar. The bar's
// width is set through element.style, which the CSP allows; a style="" attribute
// in the markup would be blocked by default-src 'self'.
function mostrarCategorias(categorias, total) {
  const itens = categorias.map((c) => {
    const pct = total > 0 ? Math.round((c.centavos / total) * 100) : 0;

    const li = document.createElement("li");
    const nome = document.createElement("span");
    nome.className = "cat-nome";
    nome.textContent = CATEGORIAS[c.categoria] || c.categoria;
    const valor = document.createElement("span");
    valor.className = "cat-valor";
    valor.textContent = `${reais(c.centavos)} · ${pct}%`;

    const trilho = document.createElement("span");
    trilho.className = "cat-trilho";
    trilho.setAttribute("aria-hidden", "true"); // the % in the text already says it
    const barra = document.createElement("span");
    barra.className = "cat-barra";
    barra.style.width = `${pct}%`;
    trilho.append(barra);

    li.append(nome, valor, trilho);
    return li;
  });
  $("categorias").replaceChildren(...itens);
  $("sem-categorias").hidden = categorias.length > 0;
}

function item(l) {
  const li = document.createElement("li");

  const texto = document.createElement("span");
  texto.className = "texto";
  texto.textContent = l.texto;

  // A paid bill shows what was paid; the expected amount goes to the meta line.
  const valorMostrado = l.pagamento ? l.pagamento.centavos : l.centavos;
  const valor = document.createElement("span");
  valor.className = l.tipo === "receita" ? "valor receita" : "valor";
  valor.textContent = (l.tipo === "receita" ? "+" : "") + reais(valorMostrado);

  const apagar = document.createElement("button");
  apagar.className = "apagar";
  apagar.type = "button";
  apagar.textContent = "✕";
  apagar.setAttribute("aria-label", `Apagar: ${l.texto}`);
  apagar.addEventListener("click", () => remover(l));

  const meta = document.createElement("span");
  meta.className = "meta";
  const partes = [dataBR.format(new Date(l.data)), CATEGORIAS[l.categoria] || l.categoria];
  if (l.forma) partes.push(FORMAS[l.forma] || l.forma);
  if (l.parcelas > 1) partes.push(`${l.parcelas}x de ${reais(Math.floor(l.centavos / l.parcelas))}`);
  meta.textContent = partes.join(" · ");

  li.append(texto, valor, apagar, meta);
  if (l.situacao) li.append(situacaoConta(l), acoesConta(l));
  return li;
}

const dataCurta = (iso) => dataBR.format(new Date(iso));

// One line saying where the bill stands, colored by state.
function situacaoConta(l) {
  const el = document.createElement("span");
  el.className = `situacao ${l.situacao}`;
  if (l.situacao === "paga") {
    let t = `Pago em ${dataCurta(l.pagamento.data)}`;
    if (l.pagamento.centavos !== l.centavos) t += ` · previsto ${reais(l.centavos)}`;
    el.textContent = t;
  } else if (l.situacao === "vencida") {
    el.textContent = `Vencida desde ${dataCurta(l.vencimento)}`;
  } else {
    el.textContent = `A pagar · vence ${dataCurta(l.vencimento)}`;
  }
  return el;
}

// "Pagar" opens a small form (date + amount, pre-filled with today and the
// expected amount); a paid bill offers "Desfazer pagamento" instead.
function acoesConta(l) {
  const box = document.createElement("div");
  box.className = "acoes";

  if (l.situacao === "paga") {
    const desfazer = document.createElement("button");
    desfazer.type = "button";
    desfazer.className = "secundario";
    desfazer.textContent = "Desfazer pagamento";
    desfazer.addEventListener("click", async () => {
      if (!confirm(`Marcar "${l.texto}" como não paga?`)) return;
      try {
        await api("DELETE", `/api/lancamentos/${l.id}/pagamento`);
        await atualizar();
      } catch (e) {
        $("erro").textContent = e.message;
      }
    });
    box.append(desfazer);
    return box;
  }

  const abrir = document.createElement("button");
  abrir.type = "button";
  abrir.className = "secundario";
  abrir.textContent = "Pagar";

  const form = document.createElement("form");
  form.className = "pagar";
  form.hidden = true;

  const data = document.createElement("input");
  data.type = "date";
  data.required = true;
  data.max = hojeISO();
  data.value = hojeISO();
  data.setAttribute("aria-label", "Data do pagamento");

  const quanto = document.createElement("input");
  quanto.type = "text";
  quanto.inputMode = "decimal";
  quanto.required = true;
  quanto.value = valorDigitavel(l.centavos);
  quanto.setAttribute("aria-label", "Valor pago");

  const confirmar = document.createElement("button");
  confirmar.type = "submit";
  confirmar.textContent = "Confirmar";

  const cancelar = document.createElement("button");
  cancelar.type = "button";
  cancelar.className = "secundario";
  cancelar.textContent = "Cancelar";

  // Errors appear next to the form: the main error line may be scrolled away.
  const erro = document.createElement("p");
  erro.className = "erro";
  erro.setAttribute("role", "alert");

  abrir.addEventListener("click", () => {
    abrir.hidden = true;
    form.hidden = false;
    quanto.focus();
  });
  cancelar.addEventListener("click", () => {
    form.hidden = true;
    abrir.hidden = false;
    erro.textContent = "";
  });
  form.addEventListener("submit", async (ev) => {
    ev.preventDefault();
    erro.textContent = "";
    confirmar.disabled = true;
    try {
      await api("POST", `/api/lancamentos/${l.id}/pagamento`, { data: data.value, valor: quanto.value });
      await atualizar();
    } catch (e) {
      erro.textContent = e.message;
    } finally {
      confirmar.disabled = false;
    }
  });

  form.append(data, quanto, confirmar, cancelar, erro);
  box.append(abrir, form);
  return box;
}

async function carregarLista() {
  const lista = await api("GET", "/api/lancamentos");
  // ponytail: shows only the 50 newest entries; pagination arrives when it is missed.
  $("lista").replaceChildren(...lista.slice(0, 50).map(item));
  $("vazio").hidden = lista.length > 0;
}

async function atualizar() {
  try {
    await Promise.all([carregarResumo(), carregarLista()]);
  } catch (e) {
    $("erro").textContent = e.message;
  }
}

async function remover(l) {
  if (!confirm(`Apagar "${l.texto}"?`)) return;
  try {
    await api("DELETE", `/api/lancamentos/${l.id}`);
    await atualizar();
  } catch (e) {
    $("erro").textContent = e.message;
  }
}

$("lancar").addEventListener("submit", async (ev) => {
  ev.preventDefault();
  const campo = $("texto");
  const botao = ev.target.querySelector("button");
  $("erro").textContent = "";
  botao.disabled = true;
  try {
    const eConta = $("e-conta").checked;
    await api("POST", "/api/lancamentos", {
      texto: campo.value,
      vencimento: eConta ? $("vencimento").value : "",
    });
    campo.value = "";
    // Most entries are regular: the option resets after each bill.
    $("e-conta").checked = false;
    mostrarVencimento();
    await atualizar();
  } catch (e) {
    // The text stays in the field so the user can fix it instead of retyping.
    $("erro").textContent = e.message;
  } finally {
    botao.disabled = false;
    campo.focus();
  }
});

// The due date is only asked for, and required, when the entry is a bill.
function mostrarVencimento() {
  const eConta = $("e-conta").checked;
  $("venc-campo").hidden = !eConta;
  $("vencimento").required = eConta;
  if (eConta && !$("vencimento").value) $("vencimento").value = hojeISO();
}
$("e-conta").addEventListener("change", mostrarVencimento);

$("mes").value = mesAtual();
$("mes").addEventListener("change", () => carregarResumo().catch((e) => { $("erro").textContent = e.message; }));
atualizar();
