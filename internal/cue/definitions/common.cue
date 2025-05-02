package definitions

import schemas "turingpi.com/schemas/schemas"

// Wait action definitions
#WaitParams: { 
    // Number of seconds to wait
    seconds: int & >0 
}

// #WaitAction pauses workflow execution for a specified duration
#WaitAction: schemas.#Action & { 
    type: "common:wait"
    params: #WaitParams
} 