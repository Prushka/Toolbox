#include <Arduino.h>
#include <HID.h>
#include <Keyboard.h>
#include <Mouse.h>

namespace {

constexpr uint8_t kRequestMagic = 0xA5;
constexpr uint8_t kResponseMagic = 0x5A;
constexpr uint8_t kProtocolVersion = 1;
constexpr uint8_t kMaximumPayload = 64;
constexpr uint32_t kWatchdogMilliseconds = 30000UL;
constexpr uint16_t kPartialFrameTimeoutMilliseconds = 250;

enum Command : uint8_t {
  kPing = 0x01,
  kInfo = 0x02,
  kKeyDown = 0x10,
  kKeyUp = 0x11,
  kKeyboardReset = 0x12,
  kTypeASCII = 0x13,
  kMouseMove = 0x20,
  kMouseAbsolute = 0x21,
  kMouseDown = 0x22,
  kMouseUp = 0x23,
  kMouseReset = 0x24,
  kMouseMoveLinearBatch = 0x25,
  kMouseMoveRelativeBatch = 0x26,
  kReleaseAll = 0x30,
  kCycleUSB = 0x31,
};

enum Status : uint8_t {
  kOK = 0,
  kUnknownCommand = 1,
  kBadPayload = 2,
  kHIDFailure = 3,
  kBadChecksum = 4,
  kBadVersion = 5,
};

constexpr uint16_t kCapabilities =
    (1 << 0) |  // keyboard
    (1 << 1) |  // relative mouse
    (1 << 2) |  // absolute mouse
    (1 << 3) |  // horizontal wheel
    (1 << 4) |  // USB detach/attach
    (1 << 5) |  // batched linear mouse
    (1 << 6);   // batched relative mouse

// Absolute positioning and relative raw input are separate application
// collections. Report 3 never asserts buttons. Standard report 1 carries X/Y,
// the vertical wheel, and buttons 1-3; report 4 carries pan and buttons 4-5.
const uint8_t kToolboxMouseDescriptor[] PROGMEM = {
    0x05, 0x01,        // Usage Page (Generic Desktop)
    0x09, 0x02,        // Usage (Mouse)
    0xA1, 0x01,        // Collection (Application)
    0x85, 0x03,        //   Report ID (3)
    0x09, 0x01,        //   Usage (Pointer)
    0xA1, 0x00,        //   Collection (Physical)
    0x05, 0x09,        //     Usage Page (Button)
    0x19, 0x01,        //     Usage Minimum (1)
    0x29, 0x05,        //     Usage Maximum (5)
    0x15, 0x00,        //     Logical Minimum (0)
    0x25, 0x01,        //     Logical Maximum (1)
    0x95, 0x05,        //     Report Count (5)
    0x75, 0x01,        //     Report Size (1)
    0x81, 0x02,        //     Input (Data, Variable, Absolute)
    0x95, 0x01,        //     Report Count (1)
    0x75, 0x03,        //     Report Size (3)
    0x81, 0x03,        //     Input (Constant)
    0x05, 0x01,        //     Usage Page (Generic Desktop)
    0x09, 0x30,        //     Usage (X)
    0x09, 0x31,        //     Usage (Y)
    0x16, 0x00, 0x00,  //     Logical Minimum (0)
    0x26, 0xFF, 0x7F,  //     Logical Maximum (32767)
    0x75, 0x10,        //     Report Size (16)
    0x95, 0x02,        //     Report Count (2)
    0x81, 0x02,        //     Input (Data, Variable, Absolute)
    0xC0,              //   End Collection
    0xC0,              // End Collection

    0x05, 0x01,  // Usage Page (Generic Desktop)
    0x09, 0x02,  // Usage (Mouse)
    0xA1, 0x01,  // Collection (Application)
    0x85, 0x04,  //   Report ID (4)
    0x09, 0x01,  //   Usage (Pointer)
    0xA1, 0x00,  //   Collection (Physical)
    0x05, 0x09,  //     Usage Page (Button)
    0x19, 0x01,  //     Usage Minimum (1)
    0x29, 0x05,  //     Usage Maximum (5)
    0x15, 0x00,  //     Logical Minimum (0)
    0x25, 0x01,  //     Logical Maximum (1)
    0x95, 0x05,  //     Report Count (5)
    0x75, 0x01,  //     Report Size (1)
    0x81, 0x02,  //     Input (Data, Variable, Absolute)
    0x95, 0x01,  //     Report Count (1)
    0x75, 0x03,  //     Report Size (3)
    0x81, 0x03,  //     Input (Constant)
    0x05, 0x01,  //     Usage Page (Generic Desktop)
    0x09, 0x30,  //     Usage (X)
    0x09, 0x31,  //     Usage (Y)
    0x09, 0x38,  //     Usage (Wheel)
    0x15, 0x81,  //     Logical Minimum (-127)
    0x25, 0x7F,  //     Logical Maximum (127)
    0x75, 0x08,  //     Report Size (8)
    0x95, 0x03,  //     Report Count (3)
    0x81, 0x06,  //     Input (Data, Variable, Relative)
    0x05, 0x0C,  //     Usage Page (Consumer)
    0x0A, 0x38, 0x02,  // Usage (AC Pan)
    0x15, 0x81,        //     Logical Minimum (-127)
    0x25, 0x7F,        //     Logical Maximum (127)
    0x75, 0x08,        //     Report Size (8)
    0x95, 0x01,        //     Report Count (1)
    0x81, 0x06,        //     Input (Data, Variable, Relative)
    0xC0,              //   End Collection
    0xC0,              // End Collection
};

struct __attribute__((packed)) AbsoluteMouseReport {
  uint8_t buttons;
  uint16_t x;
  uint16_t y;
};

struct __attribute__((packed)) StandardMouseReport {
  uint8_t buttons;
  int8_t x;
  int8_t y;
  int8_t wheel;
};

struct __attribute__((packed)) RelativeMouseReport {
  uint8_t buttons;
  int8_t x;
  int8_t y;
  int8_t wheel;
  int8_t pan;
};

class ToolboxMouse_ {
 public:
  ToolboxMouse_() : buttons_(0) {
    static HIDSubDescriptor descriptor(kToolboxMouseDescriptor,
                                       sizeof(kToolboxMouseDescriptor));
    HID().AppendDescriptor(&descriptor);
  }

  bool move(int8_t x, int8_t y, int8_t wheel, int8_t pan) {
    if (!sendStandard(buttons_, x, y, wheel)) {
      return false;
    }
    if (pan == 0 && (buttons_ & 0x18) == 0) {
      return true;
    }
    return sendExtended(buttons_, 0, 0, 0, pan);
  }

  bool moveAbsolute(uint16_t x, uint16_t y) {
    if (x > 32767 || y > 32767) {
      return false;
    }
    const AbsoluteMouseReport report = {0, x, y};
    return HID().SendReport(3, &report, sizeof(report)) >= 0;
  }

  bool press(uint8_t buttons) {
    return setButtons(buttons_ | buttons, false);
  }

  bool release(uint8_t buttons) {
    return setButtons(buttons_ & ~buttons, false);
  }

  bool releaseAll() {
    return setButtons(0, true);
  }

 private:
  bool sendStandard(uint8_t buttons, int8_t x, int8_t y, int8_t wheel) {
    const StandardMouseReport report = {
        static_cast<uint8_t>(buttons & 0x07), x, y, wheel};
    return HID().SendReport(1, &report, sizeof(report)) >= 0;
  }

  bool sendExtended(uint8_t buttons, int8_t x, int8_t y, int8_t wheel,
                    int8_t pan) {
    const RelativeMouseReport report = {
        static_cast<uint8_t>(buttons & 0x18), x, y, wheel, pan};
    return HID().SendReport(4, &report, sizeof(report)) >= 0;
  }

  bool setButtons(uint8_t next, bool force) {
    const uint8_t previous = buttons_;
    const bool standardChanged = force || (previous & 0x07) != (next & 0x07);
    const bool extendedChanged = force || (previous & 0x18) != (next & 0x18);
    if (standardChanged && !sendStandard(next, 0, 0, 0)) {
      sendStandard(previous, 0, 0, 0);
      return false;
    }
    if (extendedChanged && !sendExtended(next, 0, 0, 0, 0)) {
      if (standardChanged) {
        sendStandard(previous, 0, 0, 0);
      }
      sendExtended(previous, 0, 0, 0, 0);
      return false;
    }
    buttons_ = next;
    return true;
  }

  uint8_t buttons_;
};

ToolboxMouse_ ToolboxMouse;
uint8_t receiveBuffer[6 + kMaximumPayload];
uint8_t receiveLength = 0;
uint8_t expectedLength = 0;
uint32_t lastValidCommandAt = 0;
uint32_t lastFrameByteAt = 0;
bool commandPortWasOpen = false;

uint8_t crc8(const uint8_t* data, uint8_t length) {
  uint8_t crc = 0;
  for (uint8_t index = 0; index < length; ++index) {
    crc ^= data[index];
    for (uint8_t bit = 0; bit < 8; ++bit) {
      crc = (crc & 0x80) ? static_cast<uint8_t>((crc << 1) ^ 0x07)
                         : static_cast<uint8_t>(crc << 1);
    }
  }
  return crc;
}

bool releaseAll() {
  Keyboard.releaseAll();
  return ToolboxMouse.releaseAll();
}

bool isSupportedKey(uint8_t key) {
  return (key >= 0x20 && key <= 0x7E) ||
         (key >= 0x80 && key <= 0x87) ||
         (key >= 0xB0 && key <= 0xB3) ||
         (key >= 0xC1 && key <= 0xEB) || key == 0xED ||
         (key >= 0xF0 && key <= 0xFB);
}

bool isModifierKey(uint8_t key) {
  return key >= 0x80 && key <= 0x87;
}

bool validateKeyPayload(const uint8_t* payload, uint8_t payloadLength) {
  uint8_t nonModifiers = 0;
  for (uint8_t index = 0; index < payloadLength; ++index) {
    if (!isSupportedKey(payload[index])) {
      return false;
    }
    for (uint8_t previous = 0; previous < index; ++previous) {
      if (payload[previous] == payload[index]) {
        return false;
      }
    }
    if (!isModifierKey(payload[index]) && ++nonModifiers > 6) {
      return false;
    }
  }
  return true;
}

bool validateLinearMouseBatchPayload(const uint8_t* payload,
                                     uint8_t payloadLength) {
  if (payloadLength == 0 || (payloadLength & 1) != 0) {
    return false;
  }
  for (uint8_t index = 0; index < payloadLength; index += 2) {
    const int8_t x = static_cast<int8_t>(payload[index]);
    const int8_t y = static_cast<int8_t>(payload[index + 1]);
    if (x < -4 || x > 4 || y < -4 || y > 4) {
      return false;
    }
  }
  return true;
}

bool validateRelativeMouseBatchPayload(const uint8_t* payload,
                                       uint8_t payloadLength) {
  if (payloadLength == 0 || (payloadLength & 1) != 0) {
    return false;
  }
  for (uint8_t index = 0; index < payloadLength; index += 2) {
    if (payload[index] == 0x80 || payload[index + 1] == 0x80) {
      return false;
    }
  }
  return true;
}

void sendResponse(uint8_t sequence, Status status, const uint8_t* payload,
                  uint8_t payloadLength) {
  uint8_t response[6 + kMaximumPayload];
  response[0] = kResponseMagic;
  response[1] = kProtocolVersion;
  response[2] = sequence;
  response[3] = status;
  response[4] = payloadLength;
  if (payloadLength > 0 && payload != nullptr) {
    memcpy(response + 5, payload, payloadLength);
  }
  response[5 + payloadLength] = crc8(response, 5 + payloadLength);
  Serial.write(response, 6 + payloadLength);
}

Status runCommand(uint8_t command, const uint8_t* payload,
                  uint8_t payloadLength, uint8_t* response,
                  uint8_t* responseLength) {
  *responseLength = 0;
  switch (command) {
    case kPing:
      return payloadLength == 0 ? kOK : kBadPayload;

    case kInfo:
      if (payloadLength != 0) {
        return kBadPayload;
      }
      response[0] = 1;  // firmware major
      response[1] = 8;  // firmware minor
      response[2] = kProtocolVersion;
      response[3] = lowByte(kCapabilities);
      response[4] = highByte(kCapabilities);
      response[5] = kMaximumPayload;
      response[6] = static_cast<uint8_t>(kWatchdogMilliseconds / 1000UL);
      *responseLength = 7;
      return kOK;

    case kKeyDown:
      if (payloadLength == 0 || !validateKeyPayload(payload, payloadLength)) {
        return kBadPayload;
      }
      for (uint8_t index = 0; index < payloadLength; ++index) {
        if (Keyboard.press(payload[index]) == 0) {
          Keyboard.releaseAll();
          return kHIDFailure;
        }
      }
      return kOK;

    case kKeyUp:
      if (payloadLength == 0 || !validateKeyPayload(payload, payloadLength)) {
        return kBadPayload;
      }
      for (uint8_t index = 0; index < payloadLength; ++index) {
        Keyboard.release(payload[index]);
      }
      return kOK;

    case kKeyboardReset:
      if (payloadLength != 0) {
        return kBadPayload;
      }
      Keyboard.releaseAll();
      return kOK;

    case kTypeASCII:
      if (payloadLength == 0) {
        return kBadPayload;
      }
      for (uint8_t index = 0; index < payloadLength; ++index) {
        if (payload[index] < 0x20 || payload[index] > 0x7E) {
          return kBadPayload;
        }
      }
      for (uint8_t index = 0; index < payloadLength; ++index) {
        if (Keyboard.write(payload[index]) == 0) {
          Keyboard.releaseAll();
          return kHIDFailure;
        }
      }
      return kOK;

    case kMouseMove:
      if (payloadLength != 4) {
        return kBadPayload;
      }
      return ToolboxMouse.move(static_cast<int8_t>(payload[0]),
                               static_cast<int8_t>(payload[1]),
                               static_cast<int8_t>(payload[2]),
                               static_cast<int8_t>(payload[3]))
                 ? kOK
                 : kHIDFailure;

    case kMouseAbsolute: {
      if (payloadLength != 4) {
        return kBadPayload;
      }
      const uint16_t x = static_cast<uint16_t>(payload[0]) |
                         (static_cast<uint16_t>(payload[1]) << 8);
      const uint16_t y = static_cast<uint16_t>(payload[2]) |
                         (static_cast<uint16_t>(payload[3]) << 8);
      return ToolboxMouse.moveAbsolute(x, y) ? kOK : kBadPayload;
    }

    case kMouseDown:
      if (payloadLength != 1 || payload[0] == 0 || (payload[0] & ~0x1F) != 0) {
        return kBadPayload;
      }
      return ToolboxMouse.press(payload[0]) ? kOK : kHIDFailure;

    case kMouseUp:
      if (payloadLength != 1 || payload[0] == 0 || (payload[0] & ~0x1F) != 0) {
        return kBadPayload;
      }
      return ToolboxMouse.release(payload[0]) ? kOK : kHIDFailure;

    case kMouseReset:
      if (payloadLength != 0) {
        return kBadPayload;
      }
      return ToolboxMouse.releaseAll() ? kOK : kHIDFailure;

    case kMouseMoveLinearBatch:
      if (!validateLinearMouseBatchPayload(payload, payloadLength)) {
        return kBadPayload;
      }
      for (uint8_t index = 0; index < payloadLength; index += 2) {
        if (!ToolboxMouse.move(static_cast<int8_t>(payload[index]),
                               static_cast<int8_t>(payload[index + 1]), 0,
                               0)) {
          return kHIDFailure;
        }
      }
      return kOK;

    case kMouseMoveRelativeBatch:
      if (!validateRelativeMouseBatchPayload(payload, payloadLength)) {
        return kBadPayload;
      }
      for (uint8_t index = 0; index < payloadLength; index += 2) {
        if (!ToolboxMouse.move(static_cast<int8_t>(payload[index]),
                               static_cast<int8_t>(payload[index + 1]), 0,
                               0)) {
          return kHIDFailure;
        }
      }
      return kOK;

    case kReleaseAll:
      if (payloadLength != 0) {
        return kBadPayload;
      }
      return releaseAll() ? kOK : kHIDFailure;

    case kCycleUSB:
      if (payloadLength != 2) {
        return kBadPayload;
      }
      {
        const uint16_t detachedFor =
            static_cast<uint16_t>(payload[0]) |
            (static_cast<uint16_t>(payload[1]) << 8);
        if (detachedFor < 250 || detachedFor > 30000) {
          return kBadPayload;
        }
      }
      return releaseAll() ? kOK : kHIDFailure;

    default:
      return kUnknownCommand;
  }
}

void processFrame() {
  const uint8_t sequence = receiveBuffer[2];
  const uint8_t command = receiveBuffer[3];
  const uint8_t payloadLength = receiveBuffer[4];

  if (crc8(receiveBuffer, expectedLength - 1) !=
      receiveBuffer[expectedLength - 1]) {
    sendResponse(sequence, kBadChecksum, nullptr, 0);
    return;
  }
  if (receiveBuffer[1] != kProtocolVersion) {
    sendResponse(sequence, kBadVersion, nullptr, 0);
    return;
  }

  uint8_t response[kMaximumPayload];
  uint8_t responseLength = 0;
  const Status status =
      runCommand(command, receiveBuffer + 5, payloadLength, response,
                 &responseLength);
  lastValidCommandAt = millis();
  sendResponse(sequence, status, response, responseLength);

  if (status == kOK && command == kCycleUSB) {
    const uint16_t detachedFor =
        static_cast<uint16_t>(receiveBuffer[5]) |
        (static_cast<uint16_t>(receiveBuffer[6]) << 8);
    Serial.flush();
    delay(75);
    // Arduino AVR 1.8.8 leaves USBDevice.detach() empty. DETACH controls the
    // ATmega32U4's physical USB pull-up and makes Windows observe a real
    // disconnect; USBDevice.attach() reinitializes USB and clears this bit.
    UDCON |= (1 << DETACH);
    delay(detachedFor);
    USBDevice.attach();
  }
}

void readCommands() {
  const uint32_t now = millis();
  if (receiveLength != 0 &&
      now - lastFrameByteAt >= kPartialFrameTimeoutMilliseconds) {
    receiveLength = 0;
    expectedLength = 0;
  }
  while (Serial.available() > 0) {
    const uint8_t value = static_cast<uint8_t>(Serial.read());
    if (receiveLength == 0 && value != kRequestMagic) {
      continue;
    }
    receiveBuffer[receiveLength++] = value;
    lastFrameByteAt = millis();

    if (receiveLength == 5) {
      if (receiveBuffer[4] > kMaximumPayload) {
        sendResponse(receiveBuffer[2], kBadPayload, nullptr, 0);
        receiveLength = 0;
        expectedLength = 0;
        continue;
      }
      expectedLength = static_cast<uint8_t>(6 + receiveBuffer[4]);
    }
    if (expectedLength != 0 && receiveLength == expectedLength) {
      processFrame();
      receiveLength = 0;
      expectedLength = 0;
    }
  }
}

}  // namespace

void setup() {
  Serial.begin(115200);
  Keyboard.begin();
  Mouse.begin();
  releaseAll();
  lastValidCommandAt = millis();
}

void loop() {
  const bool commandPortIsOpen = Serial.dtr() || Serial.rts();
  if (commandPortWasOpen && !commandPortIsOpen) {
    releaseAll();
    receiveLength = 0;
    expectedLength = 0;
  }
  commandPortWasOpen = commandPortIsOpen;
  readCommands();
  const uint32_t now = millis();
  if (now - lastValidCommandAt >= kWatchdogMilliseconds) {
    releaseAll();
    lastValidCommandAt = now;
  }
}
