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

function mesAtual() {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}`;
}

async function carregarResumo() {
  const r = await api("GET", `/api/resumo?mes=${encodeURIComponent($("mes").value)}`);
  $("receitas").textContent = reais(r.receitas);
  $("despesas").textContent = reais(r.despesas);
  $("economia").textContent = reais(r.economia);
}

function item(l) {
  const li = document.createElement("li");

  const texto = document.createElement("span");
  texto.className = "texto";
  texto.textContent = l.texto;

  const valor = document.createElement("span");
  valor.className = l.tipo === "receita" ? "valor receita" : "valor";
  valor.textContent = (l.tipo === "receita" ? "+" : "") + reais(l.centavos);

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
  return li;
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
    await api("POST", "/api/lancamentos", { texto: campo.value });
    campo.value = "";
    await atualizar();
  } catch (e) {
    // The text stays in the field so the user can fix it instead of retyping.
    $("erro").textContent = e.message;
  } finally {
    botao.disabled = false;
    campo.focus();
  }
});

$("mes").value = mesAtual();
$("mes").addEventListener("change", () => carregarResumo().catch((e) => { $("erro").textContent = e.message; }));
atualizar();
