#include <Keyboard.h>
#include <Mouse.h>

void setup() {

    // open the serial port:

    Serial.begin(9600);

    // initialize control over the keyboard:

    Keyboard.begin();
    Mouse.begin();
}

// the boards.txt in install folder didn't work, there's another copy in C:\Users\dan\AppData\Local\Arduino15\packages\arduino\hardware\avr\1.8.4
// this one worked
// vid=vendor id? pid=product id?
// Default: leonardo.build.vid=0x2341
// leonardo.build.pid=0x8036
// Logitech: leonardo.build.vid=0x046d
// leonardo.build.pid=0xc223
void loop() {

    // check for incoming serial data:

    if (Serial.available() > 0) {

        // read incoming serial data:
        char d = Serial.read();
        switch (d) {
            case '0':
                Keyboard.press(' ');
                return;
            case '1':
                Keyboard.release(' ');
                return;
            case '2':
                Mouse.press();
                return;
            case '3':
                Mouse.release();
                return;
            case '4':
                Mouse.press(MOUSE_RIGHT);
                return;
            case '5':
                Mouse.release(MOUSE_RIGHT);
                return;
            case ',':
                Keyboard.press(',');
                return;
            case '<':
                Keyboard.release(',');
                return;
            case '.':
                Keyboard.press('.');
                return;
            case '>':
                Keyboard.release('.');
                return;
            case '/':
                Keyboard.press('/');
                return;
            case '?':
                Keyboard.release('/');
                return;
            case '[':
                Keyboard.press(KEY_ESC);
                return;
            case '{':
                Keyboard.release(KEY_ESC);
                return;
            case ']':
                Keyboard.press(KEY_BACKSPACE);
                return;
            case '}':
                Keyboard.release(KEY_BACKSPACE);
                return;
            case '9':
                Mouse.release();
                Mouse.release(MOUSE_RIGHT);
                Keyboard.releaseAll();
                return;
        }
        if(isUpperCase(d)){
          Keyboard.release(d);
        }else{
          Keyboard.press(d);
        }
    }

}
