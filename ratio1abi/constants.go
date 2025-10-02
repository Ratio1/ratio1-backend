package ratio1abi

// Centralized ABI and signature constants for on-chain interactions.

// ERC20 ABI with totalSupply and balanceOf
const Erc20ABI = `[{"constant":true,"inputs":[],"name":"totalSupply","outputs":[{"name":"","type":"uint256"}],"type":"function"},
{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"}]`

// Event signature for ERC20 Transfer
const TransferEventSignature = "Transfer(address,address,uint256)"

// PoAI Manager ABI fragments
const PoaiManagerNextJobIdAbi = `[{"inputs":[],"name":"nextJobId","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]`

const PoaiManagerGetAllCspsWithOwnerAbi = `[{"inputs":[],"name":"getAllCspsWithOwner","outputs":[{"components":[{"internalType":"address","name":"cspAddress","type":"address"},{"internalType":"address","name":"cspOwner","type":"address"}],"internalType":"struct CspWithOwner[]","name":"","type":"tuple[]"}],"stateMutability":"view","type":"function"}]`

// Allocation event related
const AllocationEventSignature = "RewardsAllocatedV2(uint256,address,address,uint256)"

const AllocationLogsAbi = `[{"anonymous": false,
      "inputs": [
        {
          "indexed": true,
          "internalType": "uint256",
          "name": "jobId",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeAddress",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "address",
          "name": "nodeOwner",
          "type": "address"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "usdcAmount",
          "type": "uint256"
        }
      ],
      "name": "RewardsAllocatedV2",
      "type": "event"
    }
]`

// Burn event related
const BurnEventSignature = "TokensBurned(uint256,uint256)"

const BurnLogsAbi = `
    [{
      "anonymous": false,
      "inputs": [
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "usdcAmount",
          "type": "uint256"
        },
        {
          "indexed": false,
          "internalType": "uint256",
          "name": "r1Amount",
          "type": "uint256"
        }
      ],
      "name": "TokensBurned",
      "type": "event"
    }]
`

// Oblio event ABI
const OblioLicensesCreatedAbi = `[{"anonymous": false,
        "inputs": [
          {
            "indexed": true,
            "internalType": "address",
            "name": "to",
            "type": "address"
          },
          {
            "indexed": true,
            "internalType": "bytes32",
            "name": "invoiceUuid",
            "type": "bytes32"
          },
          {
            "indexed": false,
            "internalType": "uint256",
            "name": "tokenCount",
            "type": "uint256"
          },
          {
            "indexed": false,
            "internalType": "uint256",
            "name": "unitUsdPrice",
            "type": "uint256"
          },
          {
            "indexed": false,
            "internalType": "uint256",
            "name": "totalR1Spent",
            "type": "uint256"
          }
        ],
        "name": "LicensesCreated",
        "type": "event"
  }]`

const OblioEventSignature = "LicensesCreated(address,bytes32,uint256,uint256,uint256)"

// CSP contract ABI
const PoaiManagerTotalBalanceAbi = `[{
      "inputs": [],
      "name": "getTotalEscrowsBalance",
      "outputs": [
        {
          "internalType": "int256",
          "name": "totalBalance",
          "type": "int256"
        }
      ],
      "stateMutability": "view",
      "type": "function"
    }
]`
