// resources/gnome-shell-extension/neru-overlay@neru.dev/extension.js
// Thin, stable GNOME Shell overlay renderer for Neru. Owns the D-Bus service
// org.neru.ShellOverlay and paints rect/rounded-rect/text primitives sent by
// the Neru daemon onto a fullscreen, input-transparent St.DrawingArea.
// It contains NO overlay logic (no grid/hint math); all of that stays in Neru
// so this extension is loaded once and never needs to change as Neru evolves.

import Gio from 'gi://Gio';
import St from 'gi://St';
import Shell from 'gi://Shell';
import Pango from 'gi://Pango';
import PangoCairo from 'gi://PangoCairo';

import {Extension} from 'resource:///org/gnome/shell/extensions/extension.js';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const BUS_NAME = 'org.neru.ShellOverlay';
const OBJECT_PATH = '/org/neru/ShellOverlay';

// D-Bus contract. Neru sends a full frame of primitives as JSON via Render and
// the extension replaces the current scene with it. Geometry queries return
// JSON so the wire format stays trivial and versionable.
const IFACE = `
<node>
  <interface name="org.neru.ShellOverlay">
    <method name="Ping">
      <arg type="s" direction="out" name="info"/>
    </method>
    <method name="Render">
      <arg type="s" direction="in" name="frame_json"/>
    </method>
    <method name="Clear"/>
    <method name="Show"/>
    <method name="Hide"/>
    <method name="GetMonitors">
      <arg type="s" direction="out" name="monitors_json"/>
    </method>
    <method name="GetActiveWindowRect">
      <arg type="s" direction="out" name="rect_json"/>
    </method>
    <method name="Capture">
      <arg type="s" direction="in" name="path"/>
    </method>
  </interface>
</node>`;

const VERSION = '1';

export default class NeruOverlayExtension extends Extension {
    enable() {
        this._prims = [];
        // Origin of the union-of-monitors rectangle. Neru works in global
        // logical coordinates; the DrawingArea is positioned at this origin so
        // primitive coordinates map straight through.
        this._originX = 0;
        this._originY = 0;

        this._area = new St.DrawingArea({
            reactive: false,
            can_focus: false,
            track_hover: false,
        });
        this._area.connect('repaint', this._onRepaint.bind(this));
        this._area.visible = false;

        // Top chrome floats above application windows. The actor is reactive:false,
        // so it never joins the shell's input region and Neru's libei-injected
        // clicks pass straight through to the app underneath (modern GNOME dropped
        // the old affectsInput param; non-reactive chrome is click-through already).
        Main.layoutManager.addTopChrome(this._area, {
            affectsStruts: false,
            trackFullscreen: false,
        });

        this._sizeToMonitors();
        this._monitorsChangedId = Main.layoutManager.connect(
            'monitors-changed',
            () => this._sizeToMonitors(),
        );

        this._dbusImpl = Gio.DBusExportedObject.wrapJSObject(IFACE, this);
        this._dbusImpl.export(Gio.DBus.session, OBJECT_PATH);
        this._ownerId = Gio.bus_own_name_on_connection(
            Gio.DBus.session,
            BUS_NAME,
            Gio.BusNameOwnerFlags.REPLACE,
            null,
            null,
        );
    }

    disable() {
        if (this._ownerId) {
            Gio.bus_unown_name(this._ownerId);
            this._ownerId = 0;
        }
        if (this._dbusImpl) {
            this._dbusImpl.unexport();
            this._dbusImpl = null;
        }
        if (this._monitorsChangedId) {
            Main.layoutManager.disconnect(this._monitorsChangedId);
            this._monitorsChangedId = 0;
        }
        if (this._area) {
            Main.layoutManager.removeChrome(this._area);
            this._area.destroy();
            this._area = null;
        }
        this._prims = null;
    }

    _sizeToMonitors() {
        const monitors = Main.layoutManager.monitors;
        if (!monitors || monitors.length === 0)
            return;

        let minX = Infinity;
        let minY = Infinity;
        let maxX = -Infinity;
        let maxY = -Infinity;
        for (const m of monitors) {
            minX = Math.min(minX, m.x);
            minY = Math.min(minY, m.y);
            maxX = Math.max(maxX, m.x + m.width);
            maxY = Math.max(maxY, m.y + m.height);
        }

        this._originX = minX;
        this._originY = minY;
        if (this._area) {
            this._area.set_position(minX, minY);
            this._area.set_size(maxX - minX, maxY - minY);
            this._area.queue_repaint();
        }
    }

    // ----- D-Bus methods -----

    Ping() {
        return `neru-overlay ${VERSION}`;
    }

    Render(frameJson) {
        let frame;
        try {
            frame = JSON.parse(frameJson);
        } catch {
            return;
        }
        this._prims = Array.isArray(frame?.prims) ? frame.prims : [];
        if (this._area) {
            this._area.visible = true;
            this._area.queue_repaint();
        }
    }

    Clear() {
        this._prims = [];
        if (this._area)
            this._area.queue_repaint();
    }

    Show() {
        if (this._area) {
            this._area.visible = true;
            this._area.queue_repaint();
        }
    }

    Hide() {
        if (this._area)
            this._area.visible = false;
    }

    GetMonitors() {
        const out = [];
        const monitors = Main.layoutManager.monitors || [];
        const primaryIndex = Main.layoutManager.primaryIndex;
        for (let i = 0; i < monitors.length; i++) {
            const m = monitors[i];
            out.push({
                x: m.x,
                y: m.y,
                w: m.width,
                h: m.height,
                primary: i === primaryIndex,
                scale: m.geometry_scale ?? 1,
            });
        }
        return JSON.stringify(out);
    }

    GetActiveWindowRect() {
        const win = global.display?.focus_window ?? null;
        if (!win)
            return JSON.stringify({ok: false});

        const r = win.get_frame_rect();
        return JSON.stringify({
            ok: true,
            x: r.x,
            y: r.y,
            w: r.width,
            h: r.height,
            title: win.get_title() ?? '',
            wmclass: win.get_wm_class() ?? '',
        });
    }

    // Capture is a debug-only helper for headless verification: it writes a PNG of
    // the whole stage (overlay included) using the Shell's in-process screenshot
    // API, which is exempt from the org.gnome.Shell.Screenshot D-Bus sender check.
    // Not used by Neru at runtime.
    Capture(path) {
        try {
            const shooter = new Shell.Screenshot();
            const file = Gio.File.new_for_path(path);
            const stream = file.replace(null, false, Gio.FileCreateFlags.NONE, null);
            shooter.screenshot(true, stream, (obj, res) => {
                try {
                    obj.screenshot_finish(res);
                } catch (e) {
                    log(`neru-overlay capture failed: ${e}`);
                }
                stream.close(null);
            });
        } catch (e) {
            log(`neru-overlay capture error: ${e}`);
        }
    }

    // ----- rendering -----

    _onRepaint(area) {
        const cr = area.get_context();
        const prims = this._prims || [];

        for (const p of prims) {
            switch (p.t) {
                case 'rect':
                    this._drawRect(cr, p, false);
                    break;
                case 'rrect':
                    this._drawRect(cr, p, true);
                    break;
                case 'text':
                    this._drawText(cr, p);
                    break;
                default:
                    break;
            }
        }

        cr.$dispose();
    }

    // Translate a global primitive coordinate into actor-local space.
    _lx(x) {
        return x - this._originX;
    }

    _ly(y) {
        return y - this._originY;
    }

    _drawRect(cr, p, rounded) {
        const x = this._lx(p.x);
        const y = this._ly(p.y);
        const w = p.w;
        const h = p.h;
        const radius = rounded ? Math.min(p.r ?? 0, w / 2, h / 2) : 0;

        this._roundedPath(cr, x, y, w, h, radius);

        if (p.fill !== undefined && (p.fill & 0xff000000) !== 0) {
            setSource(cr, p.fill);
            if ((p.border !== undefined) && (p.lw ?? 0) > 0)
                cr.fillPreserve();
            else
                cr.fill();
        }

        if (p.border !== undefined && (p.lw ?? 0) > 0 && (p.border & 0xff000000) !== 0) {
            cr.setLineWidth(p.lw);
            setSource(cr, p.border);
            cr.stroke();
        } else {
            cr.newPath();
        }
    }

    _roundedPath(cr, x, y, w, h, r) {
        if (r <= 0) {
            cr.rectangle(x, y, w, h);
            return;
        }
        const deg = Math.PI / 180;
        cr.newSubPath();
        cr.arc(x + w - r, y + r, r, -90 * deg, 0);
        cr.arc(x + w - r, y + h - r, r, 0, 90 * deg);
        cr.arc(x + r, y + h - r, r, 90 * deg, 180 * deg);
        cr.arc(x + r, y + r, r, 180 * deg, 270 * deg);
        cr.closePath();
    }

    _drawText(cr, p) {
        const layout = PangoCairo.create_layout(cr);
        layout.set_text(p.text ?? '', -1);

        const desc = Pango.FontDescription.new();
        if (p.font)
            desc.set_family(p.font);
        // Neru font sizes are in device pixels (matches cairo_set_font_size on
        // the wlroots path), so use an absolute size rather than points.
        desc.set_absolute_size((p.size ?? 14) * Pango.SCALE);
        layout.set_font_description(desc);

        const [pw, ph] = layout.get_pixel_size();
        // Neru passes the center point; Pango draws from the top-left.
        const tx = this._lx(p.cx) - pw / 2;
        const ty = this._ly(p.cy) - ph / 2;

        setSource(cr, p.color ?? 0xffffffff);
        cr.moveTo(tx, ty);
        PangoCairo.show_layout(cr, layout);
    }
}

// setSource applies a 0xAARRGGBB color (the format Neru uses internally) to the
// cairo context.
function setSource(cr, argb) {
    const a = ((argb >>> 24) & 0xff) / 255;
    const r = ((argb >>> 16) & 0xff) / 255;
    const g = ((argb >>> 8) & 0xff) / 255;
    const b = (argb & 0xff) / 255;
    cr.setSourceRGBA(r, g, b, a);
}
