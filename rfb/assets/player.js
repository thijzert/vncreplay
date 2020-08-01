
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
			buttons: { Lmb: 0, Rmb: 0, Mmb: 0, Su: 0, Sd: 0 },
			indicators: { Lmb: null, Rmb: null, Mmb: null, Su: null, Sd: null },
			clicks: [],
			skin: {
				img: null,
				offsetX: 0,
				offsetY: 0,
			},
			canvas: null,
			ctx: null,
		}
		this.keycaps = null;
		this.readout = null;
		this.events = [];
	}

	PushEvent(type, time, data) {
		if ( time > this.tmax ) {
			this.tmax = time;
		}

		if ( type != "?" ) {
			this.events.push({type, time, data});
		}
	}

	Render(elt) {
		this.tmax = Math.floor( this.tmax + 250 );

		this.canvas = elt.querySelector(".-framebuffer");
		this.canvas.height = this.height;
		this.canvas.width = this.width;

		this.ctx = this.canvas.getContext("2d");

		this.pointer.canvas = elt.querySelector(".-spritelayer");
		this.pointer.canvas.height = this.height;
		this.pointer.canvas.width = this.width;
		this.pointer.ctx = this.pointer.canvas.getContext("2d");

		let mousesvg = elt.querySelector(".-vic-iodevices .-vic-mouse svg");
		if ( mousesvg ) {
			this.pointer.indicators.Lmb = mousesvg.querySelector(".Lmb");
			this.pointer.indicators.Mmb = mousesvg.querySelector(".Mmb");
			this.pointer.indicators.Rmb = mousesvg.querySelector(".Rmb");
			this.pointer.indicators.Su = mousesvg.querySelector(".Su");
			this.pointer.indicators.Sd = mousesvg.querySelector(".Sd");
		}
		let kbdsvg = elt.querySelector(".-vic-iodevices .-vic-keyboard svg");
		if ( kbdsvg ) {
			this.keycaps = kbdsvg.querySelectorAll("path.keycap");
		}

		this.readout = elt.querySelector(".-vic-iodevices .-vic-readout");

		this.powerIndicator = elt.querySelector(".-vic-indicator.-power");

		this.playbutton = elt.querySelector(".-vic-controls .-playpause");
		this.playbutton.innerText = "";
		this.playbutton.addEventListener("click", () => this.TogglePlaying());

		this.seekbar = elt.querySelector(".-vic-controls .-seek");
		this.seekbar.id = "seekbar_" + Math.random().toString(36).substr(2);
		this.seekbar.type = "range";
		this.seekbar.min = 0;
		this.seekbar.max = this.tmax;
		this.seekbar.step = 20;
		this.seekbar.value = 0;

		let seek = () => this.seek();
		this.seekbar.addEventListener("change", seek);
		this.seekbar.addEventListener("input", seek);


		this.seekbarLabel = elt.querySelector(".-vic-controls .-playtime");
		this.seekbarLabel.htmlFor = this.seekbar.id;

		this.speedknob = elt.querySelector(".-vic-controls .-speedknob");

		window.addEventListener("resize", () => this.resizeSpriteLayer());
		this.resizeSpriteLayer();

		let hashtime = 0.0;
		if ( window.location.hash.length > 3 && window.location.hash.substr(0,3) == "#t=" ) {
			hashtime = parseFloat(window.location.hash.substr(3));
		}

		this.Reset();
		this.Pause();

		if ( hashtime > 0.0 ) {
			this.setTime(hashtime * 1000.0);
		} else {
			this.powerIndicator.style.backgroundColor = null;
			this.powerIndicator.style.boxShadow = null;
		}

		this.playbutton.focus();
	}

	Reset() {
		this.eventIndex = 0;
		this.setTime(0);
		this.ctx.fillStyle = 'rgb( 0, 0, 0 )';
		this.ctx.fillRect( 0, 0, this.width, this.height );

		this.readout.innerHTML = "";

		// Get rid of the pointer
		this.pointer.X = -20;
		this.pointer.Y = -20;
		this.pointer.buttons = { Lmb: 0, Rmb: 0, Mmb: 0, Su: 0, Sd: 0 };
		this.pointer.clicks = [];
		this.blitMouse();

		this.resetKeyboardIndicators();
	}

	Play() {
		if ( this.currentTime == this.tmax ) {
			this.Reset();
		}

		this.playing = true;
		this.playbutton.innerText = "\u258c\u258c";
		this.powerIndicator.style.backgroundColor = "#31ec57";
		this.powerIndicator.style.boxShadow = "0 0 0.5rem rgb(192,255,192)";

		window.requestAnimationFrame( (time) => {
			this.lastFrame = time;
			window.requestAnimationFrame( (t) => this.nextFrame(t) );
		} );
	}

	Pause() {
		this.playing = false;
		this.playbutton.innerText = "\u25b6";
		this.powerIndicator.style.backgroundColor = "#f9ad4d";
		this.powerIndicator.style.boxShadow = "0 0 0.5rem rgb(234,219,191)";
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
			this.Pause();
		}
		this.setTime(tnew);

		this.lastFrame = time;
		if ( this.playing ) {
			window.requestAnimationFrame( (t) => this.nextFrame(t) );
		}
	}

	seek() {
		this.setTime(parseFloat(this.seekbar.value));
	}

	setTime( time ) {
		this.currentTime = time;

		window.location.hash = "#t=" + (time/1000.0).toFixed(2);

		let i, n = 0;
		for ( i = 0; i < this.events.length; i++ ) {
			if ( this.events[i].time < time ) {
				n = i+1;
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
			this.applyFramebuffer(event.data, event.time);
		} else if ( event.type == "pointerupdate" ) {
			this.applyPointerUpdate(event.data, event.time);
		} else if ( event.type == "pointer-skin" ) {
			this.applyPointerSkin(event.data, event.time);
		} else if ( event.type == "keypress" ) {
			this.applyKeyPress(event.data, event.time);
		} else if ( event.type == "keyrelease" ) {
			this.applyKeyRelease(event.data, event.time);
		} else {
			console.error("Event ", event.type, " has not been implemented");
		}
	}

	applyFramebuffer(fbdata) {
		let img = document.getElementById(fbdata.Id);
		if ( img ) {
			this.ctx.drawImage(img, 0, 0);
		}
	}

	applyPointerSkin(skin) {
		if ( skin.Default == 1 ) {
			this.pointer.skin.img = null;
		}
		if ( typeof skin.Id == "string" && skin.Id != "" ) {
			this.pointer.skin.img = document.getElementById(skin.Id);
		}
		if ( typeof skin.X == "number" ) {
			this.pointer.skin.offsetX = skin.X;
		}
		if ( typeof skin.Y == "number" ) {
			this.pointer.skin.offsetY = skin.Y;
		}
	}

	applyPointerUpdate(pdata, time) {
		this.pointer.X = pdata.X;
		this.pointer.Y = pdata.Y;

		if ( pdata.Lmb && !this.pointer.buttons.Lmb ) {
			console.log("Left click");
			this.pointer.clicks.push({ type: 0, x: pdata.X, y: pdata.Y, time: time });
		}

		for ( let k in this.pointer.buttons ) {
			if ( pdata.hasOwnProperty(k) ) {
				this.pointer.buttons[k] = pdata[k];
			}
		}
	}

	applyKeyPress(keyevent) {
		const keycode = keyevent.Key;
		this.updateKeyboardIndicator(keycode, 1);

		let named_keys = {
			0x00a0: "&#x21b5;",
			0xff08: "Bksp",
			0xff09: "Tab",
			0xff0d: "Enter",
			0xff1b: "Esc",
			0xff63: "Ins",
			0xffff: "Del",
			0xff50: "Home",
			0xff57: "End",
			0xff55: "PgUp",
			0xff56: "PgDn",
			0xff51: "Left",
			0xff52: "Up",
			0xff53: "Right",
			0xff54: "Down",
			0xffbe: "F1",
			0xffbf: "F2",
			0xffc0: "F3",
			0xffc1: "F4",
			0xffc2: "F5",
			0xffc3: "F6",
			0xffc4: "F7",
			0xffc5: "F8",
			0xffc6: "F9",
			0xffc7: "F10",
			0xffc8: "F11",
			0xffc9: "F12",
			0xffe3: "Ctrl",
			0xffe4: "Ctrl",
			0xffe7: "AltGr",
			0xffe8: "AltGr",
			0xffe9: "Alt",
			0xffea: "Alt",
		};

		if ( keycode >= 32 && keycode <= 126 ) {
			// Printable character
			this.readout.innerHTML += String.fromCharCode(keycode);
		} else if ( keycode == 0xffe1 || keycode == 0xffe1 ) {
			// Ignore shift keys - they're reflected in the char code
		} else {
			let key = document.createElement("span");
			key.classList.add("-keysym");
			if ( named_keys.hasOwnProperty(keycode) ) {
				key.innerText = named_keys[keycode];
			} else {
				key.innerText = keycode.toString(16);
			}
			this.readout.appendChild(key);

			if ( keycode == 0x00a0 || keycode == 0xff0d ) {
				this.readout.innerHTML += "\n";
			}
		}
	}

	applyKeyRelease(keyevent) {
		const keycode = keyevent.Key;
		this.updateKeyboardIndicator(keycode, 0);
	}

	updateKeyboardIndicator(keycode, state) {
		for ( let keycap of this.keycaps ) {
			let nkcc = keycap.dataset["keycode"];
			let skcc = keycap.dataset["shiftcode"];

			if ( nkcc && nkcc.length > 2 ) {
				nkcc = parseInt( nkcc.substr(2), 16 );
				if ( nkcc == keycode ) {
					this.updateIndicatorState(keycap, state);
				}
			}
			if ( skcc && skcc.length > 2 ) {
				skcc = parseInt( skcc.substr(2), 16 );
				if ( skcc == keycode ) {
					this.updateIndicatorState(keycap, state);
				}
			}
		}
	}

	updateIndicatorState(indicator, state) {
		if ( state ) {
			indicator.setAttribute("fill", "#eb795c");
		} else {
			indicator.removeAttribute("fill");
		}
	}

	resetKeyboardIndicators() {
		for ( let keycap of this.keycaps ) {
			this.updateIndicatorState(keycap, 0);
		}
	}

	resizeSpriteLayer() {
		let rect = this.canvas.getBoundingClientRect();
		this.pointer.canvas.style.width = rect.width + "px";
		this.pointer.canvas.style.height = rect.height + "px";
	}

	blitMouse() {
		this.pointer.ctx.clearRect(0, 0, this.width, this.height);

		for ( let click of this.pointer.clicks ) {
			this.drawClick(click);
		}

		if ( this.pointer.skin.img ) {
			let x = this.pointer.X - this.pointer.skin.offsetX;
			let y = this.pointer.Y - this.pointer.skin.offsetY;
			this.pointer.ctx.drawImage(this.pointer.skin.img, x, y);
		}

		for ( let k in this.pointer.buttons ) {
			if ( this.pointer.indicators[k] ) {
				this.updateIndicatorState(this.pointer.indicators[k], this.pointer.buttons[k]);
			}
		}
	}

	drawClick(click) {
		const DURATION = 800;
		const MAXWIDTH = 20.0;
		const RADIUS = 50.0;

		let trel = ( this.currentTime - click.time ) / DURATION;
		console.log(trel);
		if ( trel < 0.0 || trel > 1.0 ) {
			return;
		}

		this.pointer.ctx.strokeStyle = 'rgba( 255, 30, 30, 0.7 )';
		this.pointer.ctx.lineWidth = MAXWIDTH*trel*(1.0-trel);

		this.pointer.ctx.beginPath();
		this.pointer.ctx.ellipse(click.x, click.y, RADIUS*trel, RADIUS*trel, 0, 0, Math.PI*2);
		this.pointer.ctx.stroke();
	}
}

