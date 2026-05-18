using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class ElementControl : MonoBehaviour
{
    float speed;
    int indx;
    ObstaclesGen myGen;

    public void SetIndx(int x, ObstaclesGen _myGen)
    {
        indx = x;
        myGen = _myGen;
    }

    private void Start()
    {
        speed = myGen.speeds[indx];
    }

    private void Update()
    {
        transform.position = transform.position + new Vector3(0, 0, speed * Time.deltaTime);
    }
}
