package main

const extractionRootHelpersScript = `
	const ROOT_SELECTORS = [
		"main",
		"article[role='main']",
		"[role='main']",
		".main-wrapper",
		"#main-wrapper",
		"#main",
		".content",
		"#content",
		"#app",
		"#__next",
		"[data-main]",
		"[data-testid='main']"
	];

	function hasMeaningfulText(node) {
		const text = (node && (node.innerText || node.textContent) || "")
			.replace(/\s+/g, " ")
			.trim();
		return text.length >= 80;
	}

	function pickRoot() {
		for (const selector of ROOT_SELECTORS) {
			const candidate = document.querySelector(selector);
			if (!candidate) {
				continue;
			}
			if (!hasMeaningfulText(candidate)) {
				continue;
			}
			return candidate;
		}
		return document.body;
	}

	const root = pickRoot();
`

const markdownExtractionScript = `() => {
	if (!document || !document.body) {
		return "";
	}

	const SKIP_TAGS = new Set([
		"SCRIPT", "STYLE", "NOSCRIPT", "IFRAME", "SVG", "CANVAS",
		"FORM", "BUTTON", "INPUT", "TEXTAREA", "SELECT", "OPTION",
		"META", "LINK"
	]);
	const BLOCK_TAGS = new Set([
		"ADDRESS", "ARTICLE", "ASIDE", "BLOCKQUOTE", "DETAILS", "DIV",
		"DL", "FIELDSET", "FIGCAPTION", "FIGURE", "FOOTER", "HEADER",
		"MAIN", "NAV", "P", "SECTION"
	]);
	const tick = String.fromCharCode(96);
` + extractionRootHelpersScript + `

	function normalizeWhitespace(text) {
		return (text || "")
			.replace(/\u00a0/g, " ")
			.replace(/[ \t\f\v]+/g, " ")
			.trim();
	}

	function compactInline(text) {
		return normalizeWhitespace(text).replace(/\n+/g, " ").trim();
	}

	function cleanBlock(text) {
		return (text || "")
			.replace(/\u00a0/g, " ")
			.replace(/[ \t]+\n/g, "\n")
			.replace(/\n[ \t]+/g, "\n")
			.replace(/\n{3,}/g, "\n\n")
			.trim();
	}

	function isElementHidden(element) {
		if (!element || element.nodeType !== Node.ELEMENT_NODE) {
			return false;
		}
		if (element.hidden) {
			return true;
		}
		if ((element.getAttribute("aria-hidden") || "").toLowerCase() === "true") {
			return true;
		}
		if (element.classList && (element.classList.contains("w-condition-invisible") || element.classList.contains("hide"))) {
			return true;
		}
		const style = window.getComputedStyle(element);
		if (!style) {
			return false;
		}
		return style.display === "none" || style.visibility === "hidden";
	}

	function isHidden(node) {
		let current = node && node.nodeType === Node.ELEMENT_NODE ? node : (node && node.parentElement);
		while (current) {
			if (isElementHidden(current)) {
				return true;
			}
			current = current.parentElement;
		}
		return false;
	}

	function absoluteURL(href) {
		if (!href) {
			return "";
		}
		try {
			return new URL(href, document.baseURI).toString();
		} catch (_) {
			return href;
		}
	}

	function joinInline(parts) {
		let out = "";
		for (const partRaw of parts) {
			const part = compactInline(partRaw);
			if (!part) {
				continue;
			}
			if (!out || /\s$/.test(out) || /^[,.;:!?)]/.test(part)) {
				out += part;
			} else {
				out += " " + part;
			}
		}
		return out.trim();
	}

	function renderInline(node) {
		if (!node) {
			return "";
		}
		if (node.nodeType === Node.TEXT_NODE) {
			if (isHidden(node.parentElement)) {
				return "";
			}
			return compactInline(node.textContent || "");
		}
		if (node.nodeType !== Node.ELEMENT_NODE) {
			return "";
		}
		if (isHidden(node)) {
			return "";
		}

		const tag = node.tagName ? node.tagName.toUpperCase() : "";
		if (SKIP_TAGS.has(tag)) {
			return "";
		}

		if (tag === "BR") {
			return "\n";
		}

		if (tag === "A") {
			const text = joinInline(Array.from(node.childNodes, child => renderInline(child)));
			const href = absoluteURL(node.getAttribute("href"));
			if (!text) {
				return href;
			}
			if (!href) {
				return text;
			}
			return "[" + text + "](" + href + ")";
		}

		if (tag === "IMG") {
			const alt = compactInline(node.getAttribute("alt") || "");
			const src = absoluteURL(node.getAttribute("src"));
			if (!src) {
				return alt;
			}
			return "![" + (alt || "image") + "](" + src + ")";
		}

		if (tag === "CODE" && (!node.parentElement || node.parentElement.tagName !== "PRE")) {
			const code = compactInline(node.textContent || "");
			if (!code) {
				return "";
			}
			const cleaned = code.split(tick).join("");
			return tick + cleaned + tick;
		}

		if (tag === "STRONG" || tag === "B") {
			const text = joinInline(Array.from(node.childNodes, child => renderInline(child)));
			return text ? ("**" + text + "**") : "";
		}

		if (tag === "EM" || tag === "I") {
			const text = joinInline(Array.from(node.childNodes, child => renderInline(child)));
			return text ? ("*" + text + "*") : "";
		}

		return joinInline(Array.from(node.childNodes, child => renderInline(child)));
	}

	function renderList(listNode, depth, ordered) {
		const lines = [];
		const items = Array.from(listNode.children).filter(child => child.tagName && child.tagName.toUpperCase() === "LI");

		for (let i = 0; i < items.length; i++) {
			const item = items[i];
			const marker = ordered ? String(i + 1) + ". " : "- ";
			const indent = "  ".repeat(depth);
			const bodyParts = [];
			const nestedParts = [];

			for (const child of item.childNodes) {
				if (child.nodeType === Node.ELEMENT_NODE) {
					const childTag = child.tagName ? child.tagName.toUpperCase() : "";
					if (childTag === "UL" || childTag === "OL") {
						const nested = renderList(child, depth + 1, childTag === "OL");
						if (nested) {
							nestedParts.push(nested);
						}
						continue;
					}

					const part = BLOCK_TAGS.has(childTag) ? renderBlock(child, depth + 1) : renderInline(child);
					if (part && part.trim()) {
						bodyParts.push(part.trim());
					}
					continue;
				}

				const text = renderInline(child);
				if (text) {
					bodyParts.push(text);
				}
			}

			const body = bodyParts.join(" ").replace(/\s+/g, " ").trim();
			if (body) {
				lines.push(indent + marker + body);
			}
			for (const nested of nestedParts) {
				lines.push(nested);
			}
		}

		return lines.join("\n");
	}

	function renderTable(tableNode) {
		const rowNodes = Array.from(tableNode.querySelectorAll("tr"));
		if (rowNodes.length === 0) {
			return "";
		}

		const rows = [];
		for (const rowNode of rowNodes) {
			const cells = Array.from(rowNode.children)
				.filter(cell => cell.tagName && (cell.tagName.toUpperCase() === "TH" || cell.tagName.toUpperCase() === "TD"))
				.map(cell => compactInline((cell.innerText || cell.textContent || "")).replace(/\|/g, "\\|"));
			if (cells.length > 0) {
				rows.push(cells);
			}
		}

		if (rows.length === 0) {
			return "";
		}

		let maxCols = 0;
		for (const row of rows) {
			if (row.length > maxCols) {
				maxCols = row.length;
			}
		}
		if (maxCols === 0) {
			return "";
		}

		for (const row of rows) {
			while (row.length < maxCols) {
				row.push("");
			}
		}

		const header = rows[0];
		const separator = new Array(maxCols).fill("---");
		const lines = [];
		lines.push("| " + header.join(" | ") + " |");
		lines.push("| " + separator.join(" | ") + " |");
		for (let i = 1; i < rows.length; i++) {
			lines.push("| " + rows[i].join(" | ") + " |");
		}

		return lines.join("\n");
	}

	function renderBlock(node, depth) {
		if (!node) {
			return "";
		}
		if (node.nodeType === Node.TEXT_NODE) {
			if (isHidden(node.parentElement)) {
				return "";
			}
			return compactInline(node.textContent || "");
		}
		if (node.nodeType !== Node.ELEMENT_NODE) {
			return "";
		}
		if (isHidden(node)) {
			return "";
		}

		const tag = node.tagName ? node.tagName.toUpperCase() : "";
		if (SKIP_TAGS.has(tag)) {
			return "";
		}

		if (/^H[1-6]$/.test(tag)) {
			const level = parseInt(tag.slice(1), 10);
			const text = joinInline(Array.from(node.childNodes, child => renderInline(child)));
			if (!text) {
				return "";
			}
			return "#".repeat(level) + " " + text;
		}

		if (tag === "A") {
			const href = absoluteURL(node.getAttribute("href"));
			if (!href) {
				return joinInline(Array.from(node.childNodes, child => renderInline(child)));
			}

			const headingNode = node.querySelector("h1, h2, h3, h4, h5, h6");
			if (!headingNode) {
				return renderInline(node);
			}

			const title = compactInline(headingNode.innerText || headingNode.textContent || "");
			if (!title) {
				return renderInline(node);
			}

			let details = compactInline(node.innerText || node.textContent || "");
			if (details.startsWith(title)) {
				details = details.slice(title.length).trim();
			} else {
				details = details.replace(title, "").trim();
			}

			if (!details) {
				return "[" + title + "](" + href + ")";
			}

			return "[" + title + "](" + href + ")\n\n" + details;
		}

		if (tag === "P") {
			return joinInline(Array.from(node.childNodes, child => renderInline(child)));
		}

		if (tag === "HR") {
			return "---";
		}

		if (tag === "PRE") {
			const code = (node.innerText || node.textContent || "").replace(/\u00a0/g, " ").replace(/\n+$/, "").trim();
			if (!code) {
				return "";
			}
			const fence = tick + tick + tick;
			return fence + "\n" + code + "\n" + fence;
		}

		if (tag === "BLOCKQUOTE") {
			const body = renderChildrenAsBlocks(node, depth + 1) || joinInline(Array.from(node.childNodes, child => renderInline(child)));
			if (!body) {
				return "";
			}
			return body
				.split("\n")
				.map(line => line.trim() ? "> " + line : ">")
				.join("\n");
		}

		if (tag === "UL") {
			return renderList(node, depth, false);
		}

		if (tag === "OL") {
			return renderList(node, depth, true);
		}

		if (tag === "TABLE") {
			return renderTable(node);
		}

		if (BLOCK_TAGS.has(tag)) {
			const nested = renderChildrenAsBlocks(node, depth);
			if (nested) {
				return nested;
			}
		}

		return joinInline(Array.from(node.childNodes, child => renderInline(child)));
	}

	function renderChildrenAsBlocks(parent, depth) {
		const chunks = [];
		for (const child of parent.childNodes) {
			const chunk = cleanBlock(renderBlock(child, depth));
			if (chunk) {
				chunks.push(chunk);
			}
		}
		return chunks.join("\n\n");
	}

	return renderChildrenAsBlocks(root, 0).replace(/\n{3,}/g, "\n\n").trim();
}`

const bodyTextExtractionScript = `() => {
	if (!document || !document.body) {
		return "";
	}
` + extractionRootHelpersScript + `
	return (root.innerText || "")
		.replace(/\u00a0/g, " ")
		.replace(/[ \t]+\n/g, "\n")
		.replace(/\n{3,}/g, "\n\n")
		.trim();
}`
