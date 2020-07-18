
class RFB {
	constructor(width, height) {
		this.width = width;
		this.height = height;
		this.events = [];
	}

	PushEvent(type, data) {
		if ( type != "pointerupdate" && type != "keypress" && type != "keyrelease" ) {
			this.events.push({type, data});
		}
	}

	Render(elt) {
		elt.innerHTML = "";

		this.canvas = document.createElement("canvas");
		this.canvas.height = this.height;
		this.canvas.width = this.width;

		this.canvas.style.maxHeight = "60vh";
		this.canvas.style.maxWidth = "60vw";

		this.ctx = this.canvas.getContext("2d");

		let controls = document.createElement("div");

		this.seekbar = document.createElement("input")
		this.seekbar.type = "range"
		this.seekbar.min = 0
		this.seekbar.max = this.events.length;
		this.seekbar.value = 0;

		let seek = () => this.seek();
		this.seekbar.addEventListener("change", seek)
		this.seekbar.addEventListener("input", seek)

		controls.appendChild(this.seekbar);

		elt.appendChild(this.canvas);
		elt.appendChild(controls);

		this.Reset()
	}

	Reset() {
		this.eventIndex = 0;
		this.ctx.fillStyle = 'rgb( 0, 0, 0 )';
		this.ctx.fillRect( 0, 0, this.width, this.height );
	}

	seek() {
		this.setEventIndex(this.seekbar.value);
	}

	setEventIndex(idx) {
		if ( this.eventIndex > idx ) {
			this.Reset();
		}
		for ( let i = this.eventIndex; i < idx; i++ ) {
			this.applyEvent(i);
		}
		this.eventIndex = idx;
	}

	applyEvent(idx) {
		let event = this.events[idx];
		if ( event.type == "framebuffer" ) {
			this.applyFramebuffer(event.data);
		}
	}

	applyFramebuffer(fbdata) {
		let img = document.getElementById(fbdata.id);
		if ( img ) {
			this.ctx.drawImage(img, 0, 0);
		}
	}
}

