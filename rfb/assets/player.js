
class RFB {
	constructor(width, height) {
		this.width = width;
		this.height = height;
		this.tmax = 0.0;
		this.currentTime = 0.0;
		this.playing = false;
		this.lastFrame = 0.0;
		this.events = [];
	}

	PushEvent(type, time, data) {
		if ( time > this.tmax ) {
			this.tmax = time;
		}

		if ( type != "pointerupdate" && type != "keypress" && type != "keyrelease" ) {
			this.events.push({type, time, data});
		}
	}

	Render(elt) {
		this.tmax = Math.floor( this.tmax + 250 );

		elt.innerHTML = "";

		this.canvas = document.createElement("canvas");
		this.canvas.height = this.height;
		this.canvas.width = this.width;

		this.canvas.style.maxHeight = "80vh";
		this.canvas.style.maxWidth = "80vw";

		this.ctx = this.canvas.getContext("2d");

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

		elt.appendChild(this.canvas);
		elt.appendChild(controls);

		this.Reset()
	}

	Reset() {
		this.eventIndex = 0;
		this.setTime(0);
		this.ctx.fillStyle = 'rgb( 0, 0, 0 )';
		this.ctx.fillRect( 0, 0, this.width, this.height );
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
		let tnew = this.currentTime + ( time - this.lastFrame );
		if ( tnew > this.tmax ) {
			tnew = this.tmax;
		}
		this.setTime(tnew);

		this.lastFrame = time;
		window.requestAnimationFrame( (t) => this.nextFrame(t) );
	}

	seek() {
		this.setTime(this.seekbar.value);
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
		this.seekbarLabel.innerText = "time " + z(m) + m + ":" + z(s) + s.toFixed(1);

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
		}
	}

	applyFramebuffer(fbdata) {
		let img = document.getElementById(fbdata.Id);
		if ( img ) {
			this.ctx.drawImage(img, 0, 0);
		}
	}
}

