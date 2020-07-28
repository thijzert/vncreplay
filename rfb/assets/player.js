
class RFB {
	constructor(width, height) {
		this.width = width;
		this.height = height;
		this.tmax = 0.0;
		this.currentTime = 0.0;
		this.playing = false;
		this.lastFrame = 0.0;
		this.pointer = {
			X: 0.0,
			Y: 0.0,
			canvas: null,
			ctx: null,
		}
		this.events = [];
	}

	PushEvent(type, time, data) {
		if ( time > this.tmax ) {
			this.tmax = time;
		}

		if ( type != "keypress" && type != "keyrelease" ) {
			this.events.push({type, time, data});
		}
	}

	Render(elt) {
		this.tmax = Math.floor( this.tmax + 250 );

		elt.innerHTML = "";

		let layers = document.createElement("div");
		layers.style.position = "relative";

		this.canvas = document.createElement("canvas");
		this.canvas.height = this.height;
		this.canvas.width = this.width;
		this.canvas.style.maxHeight = "80vh";
		this.canvas.style.maxWidth = "80vw";

		this.ctx = this.canvas.getContext("2d");

		layers.appendChild(this.canvas);

		this.pointer.canvas = document.createElement("canvas");
		this.pointer.canvas.height = this.height;
		this.pointer.canvas.width = this.width;
		this.pointer.canvas.style.maxHeight = "80vh";
		this.pointer.canvas.style.maxWidth = "80vw";
		this.pointer.canvas.style.position = "absolute";
		this.pointer.canvas.style.top = 0;
		this.pointer.canvas.style.left = 0;
		this.pointer.ctx = this.pointer.canvas.getContext("2d");
		layers.appendChild(this.pointer.canvas);

		let controls = document.createElement("div");

		this.playbutton = document.createElement("button");
		this.playbutton.innerText = "";
		this.playbutton.addEventListener("click", () => this.TogglePlaying());
		this.playbutton.style.width = "4rem";
		controls.appendChild(this.playbutton);

		this.seekbar = document.createElement("input");
		this.seekbar.id = "seekbar_" + Math.random().toString(36).substr(2);
		this.seekbar.type = "range";
		this.seekbar.min = 0;
		this.seekbar.max = this.tmax;
		this.seekbar.step = 0.1;
		this.seekbar.value = 0;

		let seek = () => this.seek();
		this.seekbar.addEventListener("change", seek);
		this.seekbar.addEventListener("input", seek);

		controls.appendChild(this.seekbar);

		this.seekbarLabel = document.createElement("label");
		this.seekbarLabel.htmlFor = this.seekbar.id;
		controls.appendChild(this.seekbarLabel);

		this.speedknob = this.createSpeedKnob();
		controls.appendChild(this.speedknob);

		elt.appendChild(layers);
		elt.appendChild(controls);

		this.Reset();
		this.Pause();
	}

	Reset() {
		this.eventIndex = 0;
		this.setTime(0);
		this.ctx.fillStyle = 'rgb( 0, 0, 0 )';
		this.ctx.fillRect( 0, 0, this.width, this.height );

		// Get rid of the pointer
		this.pointer.X = -20;
		this.pointer.Y = -20;
		this.blitMouse();
	}

	Play() {
		this.playing = true;
		this.playbutton.innerText = "\u258c\u258c";

		window.requestAnimationFrame( (time) => {
			this.lastFrame = time;
			window.requestAnimationFrame( (t) => this.nextFrame(t) );
		} );
	}

	Pause() {
		this.playing = false;
		this.playbutton.innerText = "\u25b6";
	}

	TogglePlaying() {
		if ( this.playing ) {
			this.Pause();
		} else {
			this.Play();
		}
	}

	nextFrame( time ) {
		if ( !this.playing ) {
			return;
		}
		let tnew = this.currentTime + ( time - this.lastFrame ) * parseFloat(this.speedknob.value);
		if ( tnew > this.tmax ) {
			tnew = this.tmax;
		}
		this.setTime(tnew);

		this.lastFrame = time;
		window.requestAnimationFrame( (t) => this.nextFrame(t) );
	}

	seek() {
		this.setTime(parseFloat(this.seekbar.value));
	}

	setTime( time ) {
		this.currentTime = time;

		let i, n = 0;
		for ( i = 0; i < this.events.length; i++ ) {
			if ( this.events[i].time < time ) {
				n = i;
			}
		}
		this.setEventIndex(n);

		let t = time / 1000;
		let m = Math.floor( t / 60 );
		let s = t - m;
		let z = (s) => s < 10 ? "0" : "";
		this.seekbarLabel.innerText = "" + z(m) + m + ":" + z(s) + s.toFixed(1);

		if ( this.seekbar.value != time ) {
			this.seekbar.value = time;
		}
	}

	setEventIndex(idx) {
		if ( this.eventIndex > idx ) {
			this.Reset();
		}
		for ( let i = this.eventIndex; i < idx; i++ ) {
			this.applyEvent(i);
		}
		if ( idx > this.eventIndex ) {
			this.blitMouse();
		}
		this.eventIndex = idx;

		this.seekbarLabel.innerText = "event " + idx;

		if ( this.seekbar.value != idx ) {
			this.seekbar.value = idx;
		}
	}

	applyEvent(idx) {
		let event = this.events[idx];
		if ( event.type == "framebuffer" ) {
			this.applyFramebuffer(event.data);
		} else if ( event.type == "pointerupdate" ) {
			this.applyPointerUpdate(event.data);
		}
	}

	applyFramebuffer(fbdata) {
		let img = document.getElementById(fbdata.Id);
		if ( img ) {
			this.ctx.drawImage(img, 0, 0);
		}
	}

	applyPointerUpdate(pdata) {
		this.pointer.X = pdata.X;
		this.pointer.Y = pdata.Y;
	}

	blitMouse() {
		this.pointer.ctx.clearRect(0, 0, this.width, this.height);

		this.pointer.ctx.fillStyle = 'rgba( 255, 30, 30, 0.7 )';
		this.pointer.ctx.beginPath();
		this.pointer.ctx.ellipse(this.pointer.X, this.pointer.Y, 3, 3, 0, 0, Math.PI*2);
		this.pointer.ctx.fill();
	}

	createSpeedKnob() {
		let o;
		let rv = document.createElement("select");

		o = document.createElement("option");
		o.value = 0.25;
		o.innerHTML = "&#x1F40C (0.25×)";
		rv.appendChild(o);

		o = document.createElement("option");
		o.value = 0.5;
		o.innerHTML = "&#x1F422 (0.5×)";
		rv.appendChild(o);

		o = document.createElement("option");
		o.value = 1.0;
		o.innerHTML = "&#x1F6B6 (1.0×)";
		o.selected = true;
		rv.appendChild(o);

		o = document.createElement("option");
		o.value = 1.5;
		o.innerHTML = "&#x1F407 (1.5×)";
		rv.appendChild(o);

		o = document.createElement("option");
		o.value = 3.0;
		o.innerHTML = "&#x1F406 (3.0×)";
		rv.appendChild(o);

		return rv;
	}
}

