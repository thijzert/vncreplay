package rfb

import "fmt"

const assetsEmbedded bool = true

var __player_2Ejs string = "\nclass RFB {\n\tconstructor(width, height) {\n\t\tthis.width = width;\n\t\tthis.height = height;\n\t\tthis.events = [];\n\t}\n\n\tPushEvent(type, time, data) {\n\t\tif ( type != \"pointerupdate\" && type != \"keypress\" && type != \"keyrelease\" ) {\n\t\t\tthis.events.push({type, data});\n\t\t}\n\t}\n\n\tRender(elt) {\n\t\telt.innerHTML = \"\";\n\n\t\tthis.canvas = document.createElement(\"canvas\");\n\t\tthis.canvas.height = this.height;\n\t\tthis.canvas.width = this.width;\n\n\t\tthis.canvas.style.maxHeight = \"80vh\";\n\t\tthis.canvas.style.maxWidth = \"80vw\";\n\n\t\tthis.ctx = this.canvas.getContext(\"2d\");\n\n\t\tlet controls = document.createElement(\"div\");\n\n\t\tthis.seekbar = document.createElement(\"input\")\n\t\tthis.seekbar.type = \"range\"\n\t\tthis.seekbar.min = 0\n\t\tthis.seekbar.max = this.events.length;\n\t\tthis.seekbar.value = 0;\n\n\t\tlet seek = () => this.seek();\n\t\tthis.seekbar.addEventListener(\"change\", seek)\n\t\tthis.seekbar.addEventListener(\"input\", seek)\n\n\t\tcontrols.appendChild(this.seekbar);\n\n\t\telt.appendChild(this.canvas);\n\t\telt.appendChild(controls);\n\n\t\tthis.Reset()\n\t}\n\n\tReset() {\n\t\tthis.eventIndex = 0;\n\t\tthis.ctx.fillStyle = 'rgb( 0, 0, 0 )';\n\t\tthis.ctx.fillRect( 0, 0, this.width, this.height );\n\t}\n\n\tseek() {\n\t\tthis.setEventIndex(this.seekbar.value);\n\t}\n\n\tsetEventIndex(idx) {\n\t\tif ( this.eventIndex > idx ) {\n\t\t\tthis.Reset();\n\t\t}\n\t\tfor ( let i = this.eventIndex; i < idx; i++ ) {\n\t\t\tthis.applyEvent(i);\n\t\t}\n\t\tthis.eventIndex = idx;\n\t}\n\n\tapplyEvent(idx) {\n\t\tlet event = this.events[idx];\n\t\tif ( event.type == \"framebuffer\" ) {\n\t\t\tthis.applyFramebuffer(event.data);\n\t\t}\n\t}\n\n\tapplyFramebuffer(fbdata) {\n\t\tlet img = document.getElementById(fbdata.Id);\n\t\tif ( img ) {\n\t\t\tthis.ctx.drawImage(img, 0, 0);\n\t\t}\n\t}\n}\n\n"

func getAsset(name string) ([]byte, error) {
	if name == "player.js" {
		return []byte(__player_2Ejs), nil
	} else {
		return nil, fmt.Errorf("asset not found")
	}
}
